package asc

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
)

func TestSetWhatsNew(t *testing.T) {
	writeTestKey(t)
	var mu sync.Mutex
	var patched map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/apps":
			_, _ = io.WriteString(w, `{"data":[{"id":"APP1","type":"apps"}]}`)
		case r.Method == http.MethodGet && r.URL.Path == "/appStoreVersions":
			_, _ = io.WriteString(w, `{"data":[{"id":"ASV1","type":"appStoreVersions"}]}`)
		case r.Method == http.MethodGet && r.URL.Path == "/appStoreVersions/ASV1/appStoreVersionLocalizations":
			_, _ = io.WriteString(w, `{"data":[{"id":"LOC1","type":"appStoreVersionLocalizations","attributes":{"locale":"en-US"}}]}`)
		case r.Method == http.MethodPatch && r.URL.Path == "/appStoreVersionLocalizations/LOC1":
			_ = json.NewDecoder(r.Body).Decode(&patched)
			w.WriteHeader(204)
		default:
			w.WriteHeader(404)
			_, _ = io.WriteString(w, r.Method+" "+r.URL.Path)
		}
	}))
	t.Cleanup(srv.Close)
	c := APIClient{BaseURL: srv.URL, HTTPClient: srv.Client()}

	if err := c.SetWhatsNew(context.Background(), WhatsNewRequest{}); err != nil {
		t.Fatalf("empty text should no-op: %v", err)
	}
	err := c.SetWhatsNew(context.Background(), WhatsNewRequest{
		BundleID: "com.example", ReleaseVersion: "1.2.3", Text: "Fixes and polish.",
	})
	if err != nil {
		t.Fatal(err)
	}
	mu.Lock()
	defer mu.Unlock()
	data, _ := patched["data"].(map[string]any)
	attrs, _ := data["attributes"].(map[string]any)
	if attrs["whatsNew"] != "Fixes and polish." {
		t.Fatalf("patched=%v", patched)
	}
}

func TestSetWhatsNewRequiresVersion(t *testing.T) {
	c := APIClient{}
	err := c.SetWhatsNew(context.Background(), WhatsNewRequest{Text: "hi", AppID: "APP1"})
	if err == nil || !strings.Contains(err.Error(), "marketing version") {
		t.Fatalf("got %v", err)
	}
}

func TestSetWhatsNewRequiresAppOrBundle(t *testing.T) {
	c := APIClient{}
	err := c.SetWhatsNew(context.Background(), WhatsNewRequest{Text: "hi", ReleaseVersion: "1.0.0"})
	if err == nil || !strings.Contains(err.Error(), "AppID or BundleID") {
		t.Fatalf("got %v", err)
	}
}
