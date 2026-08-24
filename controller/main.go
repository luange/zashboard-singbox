package main

import (
	"crypto/subtle"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

var serviceNamePattern = regexp.MustCompile(`^[A-Za-z0-9_.@-]+$`)

type supervisor interface {
	Action(string) error
	Active() (bool, error)
	Name() string
}

type commandSupervisor struct {
	kind    string
	service string
}

func (s commandSupervisor) Name() string { return s.kind }

func (s commandSupervisor) Action(action string) error {
	if action != "start" && action != "stop" && action != "restart" {
		return errors.New("unsupported action")
	}
	var command *exec.Cmd
	if s.kind == "systemd" {
		command = exec.Command("systemctl", action, s.service)
	} else {
		command = exec.Command("rc-service", s.service, action)
	}
	output, err := command.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s: %w", strings.TrimSpace(string(output)), err)
	}
	return nil
}

func (s commandSupervisor) Active() (bool, error) {
	if s.kind == "systemd" {
		err := exec.Command("systemctl", "is-active", "--quiet", s.service).Run()
		if err == nil {
			return true, nil
		}
		var exit *exec.ExitError
		if errors.As(err, &exit) {
			return false, nil
		}
		return false, err
	}
	err := exec.Command("rc-service", s.service, "status").Run()
	if err == nil {
		return true, nil
	}
	var exit *exec.ExitError
	if errors.As(err, &exit) {
		return false, nil
	}
	return false, err
}

func detectSupervisor(kind, service string) (supervisor, error) {
	if !serviceNamePattern.MatchString(service) {
		return nil, errors.New("invalid service name")
	}
	if kind == "auto" {
		if _, err := exec.LookPath("systemctl"); err == nil {
			kind = "systemd"
		} else if _, err = exec.LookPath("rc-service"); err == nil {
			kind = "openrc"
		} else {
			return nil, errors.New("neither systemd nor OpenRC was found")
		}
	}
	if kind != "systemd" && kind != "openrc" {
		return nil, errors.New("supervisor must be auto, systemd, or openrc")
	}
	return commandSupervisor{kind: kind, service: service}, nil
}

type controller struct {
	supervisor supervisor
	token      string
	core       *url.URL
	client     *http.Client
}

func (c *controller) authorized(r *http.Request) bool {
	if c.token == "" {
		return false
	}
	provided := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
	return subtle.ConstantTimeCompare([]byte(provided), []byte(c.token)) == 1
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func (c *controller) status(w http.ResponseWriter, _ *http.Request) {
	active, err := c.supervisor.Active()
	if err != nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": err.Error()})
		return
	}
	coreReachable := false
	version := ""
	request, _ := http.NewRequest(http.MethodGet, c.core.ResolveReference(&url.URL{Path: "/version"}).String(), nil)
	if response, requestErr := c.client.Do(request); requestErr == nil {
		defer response.Body.Close()
		coreReachable = response.StatusCode >= 200 && response.StatusCode < 500
		var payload struct {
			Version string `json:"version"`
		}
		_ = json.NewDecoder(response.Body).Decode(&payload)
		version = payload.Version
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"active": active, "coreReachable": coreReachable, "version": version,
		"supervisor": c.supervisor.Name(),
	})
}

func (c *controller) action(w http.ResponseWriter, r *http.Request) {
	if !c.authorized(r) {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "invalid controller token"})
		return
	}
	action := strings.TrimPrefix(r.URL.Path, "/controller/v1/")
	if err := c.supervisor.Action(action); err != nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]string{"action": action})
}

func isCoreAPIPath(path string) bool {
	for _, prefix := range []string{
		"/version", "/capabilities", "/configs", "/proxies", "/rules", "/connections",
		"/providers", "/cache", "/dns", "/logs", "/traffic", "/memory", "/group",
		"/history", "/script", "/profile", "/upgrade",
	} {
		if path == prefix || strings.HasPrefix(path, prefix+"/") {
			return true
		}
	}
	return false
}

func main() {
	listen := flag.String("listen", "127.0.0.1:9091", "controller listen address")
	coreURL := flag.String("core", "http://127.0.0.1:9090", "sing-box Clash API URL")
	service := flag.String("service", "sing-box", "allow-listed service name")
	supervisorKind := flag.String("supervisor", "auto", "auto, systemd, or openrc")
	uiDir := flag.String("ui", "./dist", "built dashboard directory")
	token := flag.String("token", os.Getenv("ZASHBOARD_CONTROLLER_TOKEN"), "controller bearer token")
	flag.Parse()

	if *token == "" {
		log.Fatal("controller token is required (flag -token or ZASHBOARD_CONTROLLER_TOKEN)")
	}
	core, err := url.Parse(*coreURL)
	if err != nil {
		log.Fatal(err)
	}
	sup, err := detectSupervisor(*supervisorKind, *service)
	if err != nil {
		log.Fatal(err)
	}
	ui, err := fs.Sub(os.DirFS(filepath.Clean(*uiDir)), ".")
	if err != nil {
		log.Fatal(err)
	}
	ctl := &controller{supervisor: sup, token: *token, core: core, client: &http.Client{Timeout: 2 * time.Second}}
	proxy := httputil.NewSingleHostReverseProxy(core)
	static := http.FileServer(http.FS(ui))

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/controller/v1/status" && r.Method == http.MethodGet:
			ctl.status(w, r)
		case strings.HasPrefix(r.URL.Path, "/controller/v1/") && r.Method == http.MethodPost:
			ctl.action(w, r)
		case isCoreAPIPath(r.URL.Path):
			proxy.ServeHTTP(w, r)
		default:
			static.ServeHTTP(w, r)
		}
	})
	log.Printf("zashboard controller listening on %s, core=%s, supervisor=%s", *listen, core, sup.Name())
	log.Fatal(http.ListenAndServe(*listen, handler))
}
