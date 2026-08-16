package asc

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	ternerrors "github.com/darkmintis/Tern/internal/errors"
)

// writeTestKey creates a throwaway P-256 key and points the ASC env at it.
func writeTestKey(t *testing.T) {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	der, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "AuthKey_TEST.p8")
	if err := os.WriteFile(path, pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der}), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv(EnvAPIKeyID, "TESTKEYID")
	t.Setenv(EnvAPIIssuerID, "TEST_ISSUER-UUID")
	t.Setenv(EnvAPIKeyPath, path)
}

type ascServer struct {
	mu             sync.Mutex
	apps           int
	builds         int
	versions       int
	createdBodies  []map[string]any
	patchedBuildID string
	statusOverride int
	noApps         bool
	hasVersion     bool
}

func newAscServer(t *testing.T, s *ascServer) *APIClient {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s.mu.Lock()
		defer s.mu.Unlock()
		if s.statusOverride > 0 {
			w.WriteHeader(s.statusOverride)
			_, _ = io.WriteString(w, `{"errors":[{"title":"denied"}]}`)
			return
		}
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/apps":
			if !strings.Contains(r.URL.RawQuery, "filter[bundleId]") {
				w.WriteHeader(400)
				return
			}
			s.apps++
			if s.noApps {
				_, _ = io.WriteString(w, `{"data":[]}`)
				return
			}
			_, _ = io.WriteString(w, `{"data":[{"id":"APP1","type":"apps","attributes":{"bundleId":"com.example"}}]}`)
		case r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/builds"):
			s.builds++
			_, _ = io.WriteString(w, `{"data":[{"id":"BUILD9","type":"builds","attributes":{"version":"17"}}]}`)
		case r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/appStoreVersions"):
			s.versions++
			if s.hasVersion {
				_, _ = io.WriteString(w, `{"data":[{"id":"ASV1","type":"appStoreVersions"}]}`)
				return
			}
			_, _ = io.WriteString(w, `{"data":[]}`)
		case r.Method == http.MethodPost && r.URL.Path == "/appStoreVersions":
			var body map[string]any
			_ = json.NewDecoder(r.Body).Decode(&body)
			s.createdBodies = append(s.createdBodies, body)
			w.WriteHeader(201)
			_, _ = io.WriteString(w, `{"data":{"id":"ASVNEW","type":"appStoreVersions"}}`)
		case r.Method == http.MethodPatch && strings.HasPrefix(r.URL.Path, "/appStoreVersions/") && strings.HasSuffix(r.URL.Path, "/relationships/build"):
			var body map[string]any
			_ = json.NewDecoder(r.Body).Decode(&body)
			if d, ok := body["data"].(map[string]any); ok {
				if id, ok := d["id"].(string); ok {
					s.patchedBuildID = id
				}
			}
			w.WriteHeader(204)
		default:
			w.WriteHeader(404)
		}
	}))
	t.Cleanup(srv.Close)
	return &APIClient{BaseURL: srv.URL, HTTPClient: srv.Client()}
}

func TestAscTokenSignsES256(t *testing.T) {
	writeTestKey(t)
	tok, err := (APIClient{}).appStoreConnectToken()
	if err != nil {
		t.Fatal(err)
	}
	parts := strings.Split(tok, ".")
	if len(parts) != 3 {
		t.Fatalf("JWT should have 3 parts, got %d", len(parts))
	}
	head, err := decodeB64(parts[0])
	if err != nil || !strings.Contains(head, "ES256") {
		t.Fatalf("header: %s %v", head, err)
	}
	payload, err := decodeB64(parts[1])
	if err != nil || !strings.Contains(payload, "appstoreconnect-v1") || !strings.Contains(payload, "TEST_ISSUER-UUID") {
		t.Fatalf("payload: %s %v", payload, err)
	}
}

func decodeB64(s string) (string, error) {
	raw, err := base64.RawURLEncoding.DecodeString(s)
	return string(raw), err
}

func TestAscLookupSuccess(t *testing.T) {
	writeTestKey(t)
	s := &ascServer{}
	c := newAscServer(t, s)
	build, err := c.Lookup(context.Background(), LookupRequest{PackageName: "com.example"})
	if err != nil {
		t.Fatal(err)
	}
	if build.AppID != "APP1" || build.BuildID != "BUILD9" || build.BuildNumber != "17" {
		t.Fatalf("got %+v", build)
	}
	if s.apps != 1 || s.builds != 1 {
		t.Fatalf("apps=%d builds=%d", s.apps, s.builds)
	}
}

func TestAscLookupNoApp(t *testing.T) {
	writeTestKey(t)
	s := &ascServer{noApps: true}
	c := newAscServer(t, s)
	_, err := c.Lookup(context.Background(), LookupRequest{PackageName: "com.missing"})
	if err == nil || ternerrors.HintOf(err) == "" || !strings.Contains(err.Error(), "no app found") {
		t.Fatalf("got %v", err)
	}
}

func TestAscLookupRequiresBundle(t *testing.T) {
	_, err := (APIClient{}).Lookup(context.Background(), LookupRequest{})
	if err == nil || !strings.Contains(err.Error(), "empty bundle id") {
		t.Fatalf("got %v", err)
	}
}

func TestAscPromoteCreatesVersion(t *testing.T) {
	writeTestKey(t)
	s := &ascServer{}
	c := newAscServer(t, s)
	msg, err := c.Promote(context.Background(), PromoteRequest{
		AppID: "APP1", BuildID: "BUILD9", BuildNumber: "17", ReleaseVersion: "1.2.3",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(msg, "Apple review is still required") {
		t.Fatalf("msg must mention Apple review: %q", msg)
	}
	if len(s.createdBodies) != 1 {
		t.Fatalf("expected 1 POST create, got %d", len(s.createdBodies))
	}
	body := s.createdBodies[0]
	if !strings.Contains(stringsMustJSON(t, body), "1.2.3") {
		t.Fatalf("create body missing release version: %v", body)
	}
}

func TestAscPromoteUpdatesExistingVersion(t *testing.T) {
	writeTestKey(t)
	s := &ascServer{hasVersion: true}
	c := newAscServer(t, s)
	_, err := c.Promote(context.Background(), PromoteRequest{
		AppID: "APP1", BuildID: "BUILD9", ReleaseVersion: "1.2.3",
	})
	if err != nil {
		t.Fatal(err)
	}
	if s.patchedBuildID != "BUILD9" {
		t.Fatalf("patched build id=%q want BUILD9", s.patchedBuildID)
	}
}

func TestAscPromoteRequiresReleaseVersion(t *testing.T) {
	_, err := (APIClient{}).Promote(context.Background(), PromoteRequest{AppID: "A", BuildID: "B"})
	if err == nil || ternerrors.HintOf(err) == "" || !strings.Contains(err.Error(), "App Store version string required") {
		t.Fatalf("got %v", err)
	}
}

func TestAscPromoteRequiresAppAndBuild(t *testing.T) {
	_, err := (APIClient{}).Promote(context.Background(), PromoteRequest{ReleaseVersion: "1.0.0"})
	if err == nil || !strings.Contains(err.Error(), "missing AppID/BuildID") {
		t.Fatalf("got %v", err)
	}
}

func TestAscHTTPErrorClassified(t *testing.T) {
	writeTestKey(t)
	s := &ascServer{statusOverride: http.StatusForbidden}
	c := newAscServer(t, s)
	_, err := c.Lookup(context.Background(), LookupRequest{PackageName: "com.example"})
	if err == nil {
		t.Fatal("expected 403 error")
	}
	if class, ok := ternerrors.AsClass(err); !ok || class != ternerrors.ClassUpload {
		t.Fatalf("class=%q", class)
	}
	if hint := ternerrors.HintOf(err); !strings.Contains(hint, "API key is missing a role") {
		t.Fatalf("hint=%q", hint)
	}
}

func TestAscLookupNoBuild(t *testing.T) {
	writeTestKey(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/builds") {
			_, _ = io.WriteString(w, `{"data":[]}`)
			return
		}
		_, _ = io.WriteString(w, `{"data":[{"id":"APP1","type":"apps"}]}`)
	}))
	t.Cleanup(srv.Close)
	c := &APIClient{BaseURL: srv.URL, HTTPClient: srv.Client()}
	_, err := c.Lookup(context.Background(), LookupRequest{PackageName: "com.example"})
	if err == nil || ternerrors.HintOf(err) == "" || !strings.Contains(err.Error(), "no succeeded TestFlight build") {
		t.Fatalf("got %v", err)
	}
}

func stringsMustJSON(t *testing.T, v any) string {
	t.Helper()
	raw, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return string(raw)
}
