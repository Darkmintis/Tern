package asc

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	ternerrors "github.com/darkmintis/Tern/internal/errors"
)

// defaultBaseURL is the App Store Connect API v1 root.
const defaultBaseURL = "https://api.appstoreconnect.apple.com/v1"

const apiTokenAudience = "appstoreconnect-v1"

// SourceBuild is the TestFlight build an App Store version should reference.
type SourceBuild struct {
	AppID       string
	BuildID     string
	BuildNumber string // e.g. "17" (ASC build.version)
}

// LookupRequest identifies the app by bundle id.
type LookupRequest struct {
	PackageName string
}

// PromoteRequest creates or updates the App Store version entry for a build
// that already sits in TestFlight.
type PromoteRequest struct {
	PackageName    string
	AppID          string
	BuildID        string
	BuildNumber    string
	ReleaseVersion string // marketing version string, e.g. "1.2.3"
}

// Lookup finds the newest succeeded build for the app's bundle id.
func (c APIClient) Lookup(ctx context.Context, req LookupRequest) (SourceBuild, error) {
	out := SourceBuild{}
	if strings.TrimSpace(req.PackageName) == "" {
		return out, ternerrors.New(ternerrors.ClassUpload, "asc: empty bundle id")
	}

	data, err := c.ascGet(ctx, "/apps?filter[bundleId]="+url.QueryEscape(req.PackageName)+"&limit=1")
	if err != nil {
		return out, err
	}
	var env ascEnvelope
	if err := json.Unmarshal(data, &env); err != nil {
		return out, ascParseError("apps response", err)
	}
	if len(env.Data) == 0 {
		return out, ternerrors.NewHint(ternerrors.ClassUpload,
			fmt.Sprintf("asc: no app found for bundle id %q", req.PackageName),
			"verify the bundle id matches App Store Connect and the app is in your team")
	}
	appID := env.Data[0].ID

	data, err = c.ascGet(ctx, "/builds?filter[app]="+appID+"&filter[processingState]=SUCCEEDED&sort=-version&limit=1")
	if err != nil {
		return out, err
	}
	if err := json.Unmarshal(data, &env); err != nil {
		return out, ascParseError("builds response", err)
	}
	if len(env.Data) == 0 {
		return out, ternerrors.NewHint(ternerrors.ClassUpload,
			"asc: no succeeded TestFlight build found",
			"upload the build to TestFlight (tern ship --to testflight) before promoting")
	}
	b := env.Data[0]
	out.AppID = appID
	out.BuildID = b.ID
	if v, ok := b.Attributes["version"].(string); ok {
		out.BuildNumber = v
	}
	return out, nil
}

// Promote makes the App Store version reference the TestFlight build. It never
// uploads an archive; Apple review still applies and is never bypassed.
func (c APIClient) Promote(ctx context.Context, req PromoteRequest) (string, error) {
	if strings.TrimSpace(req.ReleaseVersion) == "" {
		return "", ternerrors.NewHint(ternerrors.ClassUpload,
			"asc: App Store version string required",
			"pass --release-version (e.g. 1.2.3); build numbers alone are not valid App Store version strings")
	}
	if req.AppID == "" || req.BuildID == "" {
		return "", ternerrors.New(ternerrors.ClassUpload, "asc: missing AppID/BuildID (run Lookup first)")
	}

	path := "/appStoreVersions?filter[app]=" + req.AppID + "&filter[platform]=IOS&sort=-version&limit=1"
	data, err := c.ascGet(ctx, path)
	if err != nil {
		return "", err
	}
	var env ascEnvelope
	if err := json.Unmarshal(data, &env); err != nil {
		return "", ascParseError("appStoreVersions response", err)
	}

	if len(env.Data) == 0 {
		body := map[string]any{
			"data": map[string]any{
				"type": "appStoreVersions",
				"attributes": map[string]any{
					"platform":      "IOS",
					"versionString": req.ReleaseVersion,
				},
				"relationships": map[string]any{
					"app":   map[string]any{"data": map[string]any{"type": "apps", "id": req.AppID}},
					"build": map[string]any{"data": map[string]any{"type": "builds", "id": req.BuildID}},
				},
			},
		}
		if _, err := c.ascRequest(ctx, http.MethodPost, "/appStoreVersions", body); err != nil {
			return "", err
		}
	} else {
		body := map[string]any{
			"data": map[string]any{"type": "builds", "id": req.BuildID},
		}
		existing := env.Data[0].ID
		if _, err := c.ascRequest(ctx, http.MethodPatch, "/appStoreVersions/"+existing+"/relationships/build", body); err != nil {
			return "", err
		}
	}

	msg := fmt.Sprintf(
		"promoted TestFlight build %s to App Store version %s — Apple review is still required and cannot be skipped by this command; submit for review in App Store Connect",
		req.BuildNumber, req.ReleaseVersion)
	if req.BuildNumber == "" {
		msg = fmt.Sprintf(
			"promoted build %s to App Store version %s — Apple review is still required and cannot be skipped by this command; submit for review in App Store Connect",
			req.BuildID, req.ReleaseVersion)
	}
	return msg, nil
}

type ascEnvelope struct {
	Data []struct {
		ID         string         `json:"id"`
		Type       string         `json:"type"`
		Attributes map[string]any `json:"attributes"`
	} `json:"data"`
}

// ascGet is a small helper for read endpoints.
func (c APIClient) ascGet(ctx context.Context, path string) ([]byte, error) {
	return c.ascRequest(ctx, http.MethodGet, path, nil)
}

func (c APIClient) ascRequest(ctx context.Context, method, path string, body any) ([]byte, error) {
	token, err := c.appStoreConnectToken()
	if err != nil {
		return nil, err
	}
	base := strings.TrimRight(c.BaseURL, "/")
	if base == "" {
		base = defaultBaseURL
	}
	var reader io.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		if err != nil {
			return nil, ternerrors.Wrap(ternerrors.ClassUpload, "asc: encode request", err)
		}
		reader = bytes.NewReader(raw)
	}
	req, err := http.NewRequestWithContext(ctx, method, base+path, reader)
	if err != nil {
		return nil, ternerrors.Wrap(ternerrors.ClassUpload, "asc: build request", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	client := c.HTTPClient
	if client == nil {
		client = &http.Client{Timeout: 30 * time.Second}
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, ternerrors.WrapHint(ternerrors.ClassUpload,
			"asc: API request failed",
			"check your network and that api.appstoreconnect.apple.com is reachable", err)
	}
	defer func() { _ = resp.Body.Close() }()
	data, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return nil, ternerrors.Wrap(ternerrors.ClassUpload, "asc: read response", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		hint := "verify the API key has App Store Connect access and the bundle id / build exist (see docs/TROUBLESHOOTING.md)"
		if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
			hint = "the App Store Connect API key is missing a role: enable App Manager + TestFlight in Users and Access → API keys"
		}
		return nil, ternerrors.NewHint(ternerrors.ClassUpload,
			fmt.Sprintf("asc %s %s: HTTP %d — %s", method, path, resp.StatusCode, strings.TrimSpace(string(data))),
			hint)
	}
	return data, nil
}

// appStoreConnectToken builds a signed ES256 JWT from the App Store Connect
// API key (stdlib only, no new dependencies).
func (c APIClient) appStoreConnectToken() (string, error) {
	keyID := strings.TrimSpace(os.Getenv(EnvAPIKeyID))
	issuer := strings.TrimSpace(os.Getenv(EnvAPIIssuerID))
	if keyID == "" || issuer == "" {
		return "", ternerrors.NewHint(ternerrors.ClassUpload,
			"asc: set APP_STORE_CONNECT_API_KEY_ID and APP_STORE_CONNECT_API_ISSUER_ID",
			"App Store Connect → Users and Access → Integrations → Team Keys → create an API key")
	}
	keyPath := strings.TrimSpace(os.Getenv(EnvAPIKeyPath))
	if keyPath == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", ternerrors.Wrap(ternerrors.ClassUpload, "asc: home dir", err)
		}
		keyPath = filepath.Join(home, ".appstoreconnect", "private_keys", "AuthKey_"+keyID+".p8")
	}
	priv, err := loadECKey(keyPath)
	if err != nil {
		return "", err
	}

	now := time.Now().Unix()
	header := base64.RawURLEncoding.EncodeToString([]byte(fmt.Sprintf(`{"alg":"ES256","kid":%q,"typ":"JWT"}`, keyID)))
	payloadClaims, _ := json.Marshal(struct {
		Iss string `json:"iss"`
		Iat int64  `json:"iat"`
		Exp int64  `json:"exp"`
		Aud string `json:"aud"`
	}{
		Iss: issuer, Iat: now, Exp: now + 1200, Aud: apiTokenAudience,
	})
	payload := base64.RawURLEncoding.EncodeToString(payloadClaims)
	signingInput := header + "." + payload
	digest := sha256.Sum256([]byte(signingInput))
	r, s, err := ecdsa.Sign(rand.Reader, priv, digest[:])
	if err != nil {
		return "", ternerrors.WrapHint(ternerrors.ClassUpload,
			"asc: signing API key token failed",
			"confirm the .p8 is a valid App Store Connect key (ES256 secp256r1)", err)
	}
	sig := make([]byte, 64)
	r.FillBytes(sig[:32])
	s.FillBytes(sig[32:])
	return signingInput + "." + base64.RawURLEncoding.EncodeToString(sig), nil
}

// loadECKey reads a PEM-encoded EC private key (PKCS#8 or SEC1).
func loadECKey(path string) (*ecdsa.PrivateKey, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, ternerrors.WrapHint(ternerrors.ClassUpload,
			"asc: reading API key .p8 failed",
			"export APP_STORE_CONNECT_API_KEY_PATH to the downloaded .p8, or move it to ~/.appstoreconnect/private_keys/AuthKey_<KEY_ID>.p8", err)
	}
	block, _ := pem.Decode(data)
	if block == nil {
		return nil, ternerrors.New(ternerrors.ClassUpload, "asc: invalid API key file (no PEM block)")
	}
	var priv *ecdsa.PrivateKey
	switch block.Type {
	case "EC PRIVATE KEY":
		key, err := x509.ParseECPrivateKey(block.Bytes)
		if err != nil {
			return nil, ascKeyParseError(err)
		}
		priv = key
	case "PRIVATE KEY":
		key, err := x509.ParsePKCS8PrivateKey(block.Bytes)
		if err != nil {
			return nil, ascKeyParseError(err)
		}
		var ok bool
		if priv, ok = key.(*ecdsa.PrivateKey); !ok {
			return nil, ternerrors.New(ternerrors.ClassUpload, "asc: API key is not an EC (P-256) key")
		}
	default:
		return nil, ascKeyParseError(errors.New("unsupported PEM block type: " + block.Type))
	}
	if priv.Curve != elliptic.P256() {
		return nil, ternerrors.New(ternerrors.ClassUpload, "asc: API key must be P-256 (ES256)")
	}
	return priv, nil
}

func ascKeyParseError(err error) error {
	return ternerrors.WrapHint(ternerrors.ClassUpload,
		"asc: parsing API key failed",
		"download a fresh key from App Store Connect — Users and Access → Integrations → Team Keys", err)
}

func ascParseError(kind string, err error) error {
	return ternerrors.WrapHint(ternerrors.ClassUpload,
		"asc: parsing "+kind+" failed",
		"the App Store Connect API returned an unexpected shape; retry with current Tern", err)
}
