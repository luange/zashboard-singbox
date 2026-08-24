package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCoreAPIPathAllowList(t *testing.T) {
	for _, path := range []string{"/version", "/providers/proxies/airport", "/cache/dns/flush", "/connections"} {
		if !isCoreAPIPath(path) {
			t.Fatalf("expected core API path: %s", path)
		}
	}
	for _, path := range []string{"/", "/assets/index.js", "/controller/v1/status", "/providers-evil"} {
		if isCoreAPIPath(path) {
			t.Fatalf("unexpected core API path: %s", path)
		}
	}
}

func TestControllerCORSPreflight(t *testing.T) {
	request := httptest.NewRequest(http.MethodOptions, "/controller/v1/provider-overrides/airport", nil)
	recorder := httptest.NewRecorder()
	if !allowBrowserControllerAPI(recorder, request) {
		t.Fatal("OPTIONS request was not handled")
	}
	response := recorder.Result()
	if response.StatusCode != http.StatusNoContent || response.Header.Get("Access-Control-Allow-Origin") != "*" {
		t.Fatalf("unexpected CORS response: status=%d headers=%v", response.StatusCode, response.Header)
	}
}

func TestControllerAuthorization(t *testing.T) {
	controller := &controller{token: "test-token"}
	request, _ := http.NewRequest(http.MethodPost, "/controller/v1/restart", nil)
	if controller.authorized(request) {
		t.Fatal("request without token must be rejected")
	}
	request.Header.Set("Authorization", "Bearer test-token")
	if !controller.authorized(request) {
		t.Fatal("matching bearer token must be accepted")
	}
}

func TestTrustedLANProviderAuthorizationRequiresSameHostBrowserOrigin(t *testing.T) {
	controller := &controller{token: "secret", trustedLANProviderUI: true}
	request := httptest.NewRequest(http.MethodGet, "http://10.30.0.115:19091/controller/v1/provider-overrides", nil)
	request.RemoteAddr = "192.168.0.2:54321"
	request.Header.Set("Origin", "http://10.30.0.115:9090")
	if !controller.authorizedProvider(request) {
		t.Fatal("same-host private browser origin should be accepted")
	}
	request.Header.Set("Origin", "https://attacker.example")
	if controller.authorizedProvider(request) {
		t.Fatal("different browser origin must be rejected")
	}
	request.Header.Del("Origin")
	if controller.authorizedProvider(request) {
		t.Fatal("non-browser request without token must be rejected")
	}
}

func TestServiceNameValidation(t *testing.T) {
	for _, name := range []string{"sing-box", "singbox.service", "singbox@edge"} {
		if !serviceNamePattern.MatchString(name) {
			t.Fatalf("valid service rejected: %s", name)
		}
	}
	for _, name := range []string{"", "sing-box;reboot", "../service", "sing box"} {
		if serviceNamePattern.MatchString(name) {
			t.Fatalf("unsafe service accepted: %s", name)
		}
	}
}
