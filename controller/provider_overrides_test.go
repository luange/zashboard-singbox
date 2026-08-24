package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
)

func TestRenderProviderOverridesUpdatesAndInjectsProviders(t *testing.T) {
	base := []byte(`{
  "providers":[{"type":"remote","tag":"airport","url":"https://old.example/sub","update_interval":"24h"}],
  "outbounds":[
    {"type":"smart","tag":"HK","providers":["airport"]},
    {"type":"smart","tag":"US","providers":["airport"]}
  ]
}`)
	overrides := []byte(`{
  "version":1,
  "providers":{
    "airport":{"definition":{"update_interval":"6h"},"attach_to":["HK"]},
    "backup":{"definition":{"type":"remote","url":"https://new.example/sub","format":"clash"},"attach_to":["HK","US"]}
  }
}`)
	rendered, err := renderProviderOverrides(base, overrides)
	if err != nil {
		t.Fatal(err)
	}
	var config map[string]any
	if err = json.Unmarshal(rendered, &config); err != nil {
		t.Fatal(err)
	}
	providers := config["providers"].([]any)
	if len(providers) != 2 {
		t.Fatalf("expected two providers, got %d", len(providers))
	}
	airport := providers[0].(map[string]any)
	if airport["url"] != "https://old.example/sub" || airport["update_interval"] != "6h" {
		t.Fatalf("base fields were not preserved and overwritten correctly: %#v", airport)
	}
	outbounds := config["outbounds"].([]any)
	hk := outbounds[0].(map[string]any)["providers"].([]any)
	us := outbounds[1].(map[string]any)["providers"].([]any)
	if len(hk) != 2 || hk[0] != "airport" || hk[1] != "backup" {
		t.Fatalf("unexpected HK providers: %#v", hk)
	}
	if len(us) != 1 || us[0] != "backup" {
		t.Fatalf("unexpected US providers: %#v", us)
	}
}

func TestRedactProviderOverrideSecrets(t *testing.T) {
	redacted := redactProviderOverride(providerOverride{Definition: map[string]any{
		"url":     "https://user:pass@example.com/sub?token=secret",
		"headers": map[string]any{"Authorization": "secret"},
		"format":  "clash",
		"health_check": map[string]any{
			"url": "https://example.com/generate_204?token=secret#fragment", "interval": "10m",
		},
	}})
	if _, exists := redacted.Definition["url"]; exists {
		t.Fatal("URL must not be returned")
	}
	if _, exists := redacted.Definition["headers"]; exists {
		t.Fatal("headers must not be returned")
	}
	if redacted.Definition["url_configured"] != true || redacted.Definition["headers_configured"] != true {
		t.Fatal("configured flags missing")
	}
	health := redacted.Definition["health_check"].(map[string]any)
	if health["url"] != "https://example.com/generate_204" {
		t.Fatalf("health check query leaked: %v", health["url"])
	}
}

func TestRenderProviderOverridesRejectsUnsupportedVersion(t *testing.T) {
	_, err := renderProviderOverrides([]byte(`{"providers":[],"outbounds":[]}`), []byte(`{"version":2,"providers":{}}`))
	if err == nil {
		t.Fatal("expected unsupported version error")
	}
}

func TestRenderProviderOverridesRejectsNewProviderWithoutSource(t *testing.T) {
	_, err := renderProviderOverrides([]byte(`{"providers":[],"outbounds":[]}`), []byte(`{
  "version":1,"providers":{"broken":{"definition":{"type":"remote"}}}
}`))
	if err == nil {
		t.Fatal("new provider without URL or path must fail")
	}
}

func TestCheckRemoteProviderAcceptsSubscriptionContent(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer test" {
			t.Fatal("configured provider header was not sent")
		}
		_, _ = w.Write([]byte("proxies:\n  - name: test\n"))
	}))
	defer server.Close()
	err := checkRemoteProvider(context.Background(), map[string]any{
		"type": "remote", "url": server.URL + "/sub?token=secret",
		"headers": map[string]any{"Authorization": "Bearer test"},
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestCheckRemoteProviderRejectsHTMLAndEmpty(t *testing.T) {
	for _, body := range []string{"", "<!doctype html><title>blocked</title>"} {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			_, _ = w.Write([]byte(body))
		}))
		err := checkRemoteProvider(context.Background(), map[string]any{"type": "remote", "url": server.URL})
		server.Close()
		if err == nil {
			t.Fatalf("expected response %q to be rejected", body)
		}
	}
}

func TestProviderOverrideViewIncludesRedactedBaseMetadata(t *testing.T) {
	path := t.TempDir() + "/config.json"
	base := `{"providers":[{"type":"remote","tag":"airport","url":"https://example.com/sub?token=secret","format":"clash"}],"outbounds":[{"type":"smart","tag":"US","providers":["airport"]}]}`
	if err := os.WriteFile(path, []byte(base), 0o600); err != nil {
		t.Fatal(err)
	}
	manager := providerOverrideManager{baseConfig: path}
	views, err := manager.view(emptyProviderOverrideDocument())
	if err != nil {
		t.Fatal(err)
	}
	provider := views["airport"]
	if provider.Overridden || provider.Definition["url_configured"] != true {
		t.Fatalf("unexpected provider view: %#v", provider)
	}
	if _, exists := provider.Definition["url"]; exists {
		t.Fatal("base subscription URL leaked")
	}
	if len(provider.AttachTo) != 1 || provider.AttachTo[0] != "US" {
		t.Fatalf("base attachments missing: %#v", provider.AttachTo)
	}
}
