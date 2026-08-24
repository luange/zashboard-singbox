package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

const providerOverrideVersion = 1

type providerOverride struct {
	Definition map[string]any `json:"definition"`
	AttachTo   []string       `json:"attach_to,omitempty"`
}

type providerOverrideDocument struct {
	Version   int                         `json:"version"`
	Providers map[string]providerOverride `json:"providers"`
}

type providerOverrideManager struct {
	path    string
	builder string
	check   func(context.Context, map[string]any) error
	mu      sync.Mutex
}

func emptyProviderOverrideDocument() providerOverrideDocument {
	return providerOverrideDocument{Version: providerOverrideVersion, Providers: map[string]providerOverride{}}
}

func (m *providerOverrideManager) load() (providerOverrideDocument, error) {
	document := emptyProviderOverrideDocument()
	data, err := os.ReadFile(m.path)
	if errors.Is(err, os.ErrNotExist) {
		return document, nil
	}
	if err != nil {
		return document, err
	}
	if err = json.Unmarshal(data, &document); err != nil {
		return document, fmt.Errorf("decode provider overrides: %w", err)
	}
	if document.Version != providerOverrideVersion || document.Providers == nil {
		return document, errors.New("unsupported provider override document")
	}
	return document, nil
}

func validateProviderOverride(tag string, override providerOverride) error {
	if !serviceNamePattern.MatchString(tag) {
		return errors.New("invalid provider tag")
	}
	if override.Definition == nil {
		return errors.New("provider definition is required")
	}
	if definitionTag, exists := override.Definition["tag"]; exists && definitionTag != tag {
		return errors.New("definition tag must match request path")
	}
	if providerType, exists := override.Definition["type"]; exists && providerType != "remote" && providerType != "local" {
		return errors.New("provider type must be remote or local")
	}
	if rawURL, exists := override.Definition["url"]; exists && rawURL != "" {
		parsed, parseErr := url.Parse(fmt.Sprint(rawURL))
		if parseErr != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" {
			return errors.New("provider URL must be HTTP or HTTPS")
		}
	}
	seen := map[string]bool{}
	for _, group := range override.AttachTo {
		if !serviceNamePattern.MatchString(group) || seen[group] {
			return errors.New("invalid or duplicate attach_to group")
		}
		seen[group] = true
	}
	return nil
}

func writeProviderOverrideDocument(path string, document providerOverrideDocument) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(document, "", "  ")
	if err != nil {
		return err
	}
	temporary, err := os.CreateTemp(filepath.Dir(path), ".provider-overrides-*")
	if err != nil {
		return err
	}
	temporaryName := temporary.Name()
	defer os.Remove(temporaryName)
	if err = temporary.Chmod(0o600); err == nil {
		_, err = temporary.Write(append(data, '\n'))
	}
	if err == nil {
		err = temporary.Sync()
	}
	closeErr := temporary.Close()
	if err == nil {
		err = closeErr
	}
	if err != nil {
		return err
	}
	return os.Rename(temporaryName, path)
}

func (m *providerOverrideManager) mutate(mutation func(*providerOverrideDocument) error) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	document, err := m.load()
	if err != nil {
		return err
	}
	oldData, oldErr := os.ReadFile(m.path)
	if oldErr != nil && !errors.Is(oldErr, os.ErrNotExist) {
		return oldErr
	}
	if err = mutation(&document); err != nil {
		return err
	}
	if err = writeProviderOverrideDocument(m.path, document); err != nil {
		return err
	}
	if m.builder != "" {
		if output, buildErr := exec.Command(m.builder).CombinedOutput(); buildErr != nil {
			if oldErr == nil {
				_ = os.WriteFile(m.path, oldData, 0o600)
			} else {
				_ = os.Remove(m.path)
			}
			return fmt.Errorf("runtime config rejected override: %s: %w", strings.TrimSpace(string(output)), buildErr)
		}
	}
	return nil
}

func checkRemoteProvider(ctx context.Context, definition map[string]any) error {
	if definition["type"] == "local" {
		return nil
	}
	rawURL, _ := definition["url"].(string)
	if rawURL == "" {
		return nil
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return errors.New("invalid subscription address")
	}
	request.Header.Set("User-Agent", "sing-box-provider-check/1")
	request.Header.Set("Accept", "application/json, application/yaml, text/yaml, text/plain, */*")
	request.Header.Set("Range", "bytes=0-1048575")
	if headers, ok := definition["headers"].(map[string]any); ok {
		for name, rawValue := range headers {
			if !isSafeProviderHeader(name) {
				continue
			}
			switch value := rawValue.(type) {
			case string:
				request.Header.Set(name, value)
			case []any:
				for _, item := range value {
					request.Header.Add(name, fmt.Sprint(item))
				}
			}
		}
	}
	client := &http.Client{Timeout: 12 * time.Second}
	response, err := client.Do(request)
	if err != nil {
		return errors.New("subscription address is unreachable")
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return fmt.Errorf("subscription server returned HTTP %d", response.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(response.Body, 1<<20+1))
	if err != nil {
		return errors.New("subscription response could not be read")
	}
	if len(body) == 0 {
		return errors.New("subscription response is empty")
	}
	trimmed := bytes.TrimSpace(body)
	if len(trimmed) == 0 {
		return errors.New("subscription response is empty")
	}
	lower := bytes.ToLower(trimmed[:min(len(trimmed), 256)])
	if bytes.HasPrefix(lower, []byte("<!doctype html")) || bytes.HasPrefix(lower, []byte("<html")) {
		return errors.New("subscription server returned an HTML page")
	}
	return nil
}

func isSafeProviderHeader(name string) bool {
	canonical := http.CanonicalHeaderKey(name)
	return canonical != "Host" && canonical != "Content-Length" && canonical != "Connection" && canonical != "Transfer-Encoding"
}

func redactProviderOverride(override providerOverride) providerOverride {
	redacted := providerOverride{Definition: map[string]any{}, AttachTo: append([]string(nil), override.AttachTo...)}
	for key, value := range override.Definition {
		if key == "url" {
			redacted.Definition["url_configured"] = value != ""
			continue
		}
		if key == "headers" {
			redacted.Definition["headers_configured"] = value != nil
			continue
		}
		if key == "health_check" {
			if health, ok := value.(map[string]any); ok {
				redactedHealth := deepMerge(nil, health)
				if rawURL, exists := redactedHealth["url"]; exists {
					redactedHealth["url"] = redactURLQuery(fmt.Sprint(rawURL))
				}
				redacted.Definition[key] = redactedHealth
				continue
			}
		}
		redacted.Definition[key] = value
	}
	return redacted
}

func redactURLQuery(raw string) string {
	parsed, err := url.Parse(raw)
	if err != nil {
		return ""
	}
	parsed.RawQuery = ""
	parsed.ForceQuery = false
	parsed.Fragment = ""
	return parsed.String()
}

func deepMerge(base, overlay map[string]any) map[string]any {
	result := map[string]any{}
	for key, value := range base {
		result[key] = value
	}
	for key, value := range overlay {
		if nestedOverlay, ok := value.(map[string]any); ok {
			if nestedBase, nestedOK := result[key].(map[string]any); nestedOK {
				result[key] = deepMerge(nestedBase, nestedOverlay)
				continue
			}
		}
		result[key] = value
	}
	return result
}

func renderProviderOverrides(baseData, overrideData []byte) ([]byte, error) {
	var config map[string]any
	if err := json.Unmarshal(baseData, &config); err != nil {
		return nil, fmt.Errorf("decode base config: %w", err)
	}
	document := emptyProviderOverrideDocument()
	if len(overrideData) > 0 {
		if err := json.Unmarshal(overrideData, &document); err != nil {
			return nil, fmt.Errorf("decode provider overrides: %w", err)
		}
		if document.Version != providerOverrideVersion || document.Providers == nil {
			return nil, errors.New("unsupported provider override document")
		}
	}
	providers, _ := config["providers"].([]any)
	providerIndex := map[string]int{}
	for index, rawProvider := range providers {
		if provider, ok := rawProvider.(map[string]any); ok {
			if tag, tagOK := provider["tag"].(string); tagOK {
				providerIndex[tag] = index
			}
		}
	}
	tags := make([]string, 0, len(document.Providers))
	for tag := range document.Providers {
		tags = append(tags, tag)
	}
	sort.Strings(tags)
	for _, tag := range tags {
		override := document.Providers[tag]
		if err := validateProviderOverride(tag, override); err != nil {
			return nil, fmt.Errorf("provider %s: %w", tag, err)
		}
		definition := map[string]any{}
		if index, exists := providerIndex[tag]; exists {
			definition, _ = providers[index].(map[string]any)
			definition = deepMerge(definition, override.Definition)
			definition["tag"] = tag
			providers[index] = definition
		} else {
			definition = deepMerge(nil, override.Definition)
			definition["tag"] = tag
			if definition["type"] == nil {
				definition["type"] = "remote"
			}
			if definition["url"] == nil && definition["path"] == nil {
				return nil, fmt.Errorf("new provider %s requires url or path", tag)
			}
			providerIndex[tag] = len(providers)
			providers = append(providers, definition)
		}
		if override.AttachTo != nil {
			wanted := map[string]bool{}
			for _, group := range override.AttachTo {
				wanted[group] = true
			}
			outbounds, _ := config["outbounds"].([]any)
			for _, rawOutbound := range outbounds {
				outbound, ok := rawOutbound.(map[string]any)
				if !ok {
					continue
				}
				group, _ := outbound["tag"].(string)
				rawProviders, hasProviders := outbound["providers"].([]any)
				if !hasProviders && !wanted[group] {
					continue
				}
				updated := make([]any, 0, len(rawProviders)+1)
				for _, raw := range rawProviders {
					if raw != tag {
						updated = append(updated, raw)
					}
				}
				if wanted[group] {
					updated = append(updated, tag)
				}
				outbound["providers"] = updated
			}
		}
	}
	config["providers"] = providers
	return json.MarshalIndent(config, "", "  ")
}
