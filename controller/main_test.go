package main

import (
	"net/http"
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
