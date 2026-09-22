package asc

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	ternerrors "github.com/darkmintis/Tern/internal/errors"
)

// WhatsNewRequest sets App Store Connect "What's New" for a marketing version.
type WhatsNewRequest struct {
	AppID          string // ASC app id (preferred)
	BundleID       string // used to look up AppID when empty
	ReleaseVersion string // e.g. 1.2.3
	Locale         string // e.g. en-US
	Text           string
}

// SetWhatsNew patches appStoreVersionLocalizations.whatsNew for the given version.
// Best-effort: callers may keep writing the local meta file when this fails.
func (c APIClient) SetWhatsNew(ctx context.Context, req WhatsNewRequest) error {
	text := strings.TrimSpace(req.Text)
	if text == "" {
		return nil
	}
	locale := strings.TrimSpace(req.Locale)
	if locale == "" {
		locale = "en-US"
	}
	version := strings.TrimSpace(req.ReleaseVersion)
	if version == "" {
		return ternerrors.New(ternerrors.ClassUpload, "asc: What's New requires a marketing version")
	}
	appID := strings.TrimSpace(req.AppID)
	if appID == "" {
		bundle := strings.TrimSpace(req.BundleID)
		if bundle == "" {
			return ternerrors.New(ternerrors.ClassUpload, "asc: What's New requires AppID or BundleID")
		}
		id, err := c.lookupAppID(ctx, bundle)
		if err != nil {
			return err
		}
		appID = id
	}
	verID, locID, err := c.lookupVersionLocalization(ctx, appID, version, locale)
	if err != nil {
		return err
	}
	_ = verID
	body := map[string]any{
		"data": map[string]any{
			"type": "appStoreVersionLocalizations",
			"id":   locID,
			"attributes": map[string]any{
				"whatsNew": text,
			},
		},
	}
	_, err = c.ascRequest(ctx, http.MethodPatch, "/appStoreVersionLocalizations/"+locID, body)
	return err
}

func (c APIClient) lookupAppID(ctx context.Context, bundleID string) (string, error) {
	q := url.Values{}
	q.Set("filter[bundleId]", bundleID)
	raw, err := c.ascGet(ctx, "/apps?"+q.Encode())
	if err != nil {
		return "", err
	}
	var env ascEnvelope
	if err := json.Unmarshal(raw, &env); err != nil || len(env.Data) == 0 {
		return "", ternerrors.NewHint(ternerrors.ClassUpload,
			"asc: no app found for bundle "+bundleID,
			"confirm the bundle id matches App Store Connect")
	}
	return env.Data[0].ID, nil
}

func (c APIClient) lookupVersionLocalization(ctx context.Context, appID, version, locale string) (versionID, localizationID string, err error) {
	q := url.Values{}
	q.Set("filter[versionString]", version)
	q.Set("filter[app]", appID)
	raw, err := c.ascGet(ctx, "/appStoreVersions?"+q.Encode())
	if err != nil {
		return "", "", err
	}
	var env ascEnvelope
	if err := json.Unmarshal(raw, &env); err != nil || len(env.Data) == 0 {
		return "", "", ternerrors.NewHint(ternerrors.ClassUpload,
			fmt.Sprintf("asc: no App Store version %q for app %s", version, appID),
			"create the version in App Store Connect or promote a TestFlight build first")
	}
	versionID = env.Data[0].ID
	raw, err = c.ascGet(ctx, "/appStoreVersions/"+versionID+"/appStoreVersionLocalizations")
	if err != nil {
		return "", "", err
	}
	if err := json.Unmarshal(raw, &env); err != nil {
		return "", "", ternerrors.Wrap(ternerrors.ClassUpload, "asc: parse localizations", err)
	}
	want := strings.ToLower(locale)
	for _, d := range env.Data {
		loc, _ := d.Attributes["locale"].(string)
		if strings.EqualFold(loc, locale) || strings.ToLower(loc) == want {
			return versionID, d.ID, nil
		}
	}
	if len(env.Data) > 0 {
		return versionID, env.Data[0].ID, nil
	}
	return "", "", ternerrors.New(ternerrors.ClassUpload, "asc: no localizations for version "+version)
}
