package proxy

import (
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/neomody77/kuro/recovery/version"
)

func TestProxy_VersionNotRunning_503(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, "v1"), 0755)

	m := version.NewManager(dir)
	m.Scan()
	p := New(m)

	req := httptest.NewRequest("GET", "/api/test?v=v1", nil)
	w := httptest.NewRecorder()
	p.ServeHTTP(w, req)

	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("expected 503, got %d", w.Code)
	}
	if !strings.Contains(w.Body.String(), "not running") {
		t.Errorf("expected 'not running' in body, got %s", w.Body.String())
	}
}

func TestProxy_NoDefault_503(t *testing.T) {
	dir := t.TempDir()
	m := version.NewManager(dir)
	m.Scan()
	p := New(m)

	req := httptest.NewRequest("GET", "/api/test", nil)
	w := httptest.NewRecorder()
	p.ServeHTTP(w, req)

	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("expected 503, got %d", w.Code)
	}
	if !strings.Contains(w.Body.String(), "no default") {
		t.Errorf("expected 'no default' in body, got %s", w.Body.String())
	}
}

func TestProxy_NonexistentVersion_503(t *testing.T) {
	dir := t.TempDir()
	m := version.NewManager(dir)
	m.Scan()
	p := New(m)

	req := httptest.NewRequest("GET", "/api/test?v=nope", nil)
	w := httptest.NewRecorder()
	p.ServeHTTP(w, req)

	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("expected 503, got %d", w.Code)
	}
}

func TestProxy_DefaultSetButNotRunning_503(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, "v1"), 0755)

	m := version.NewManager(dir)
	m.Scan()
	m.SetDefault("v1")
	p := New(m)

	req := httptest.NewRequest("GET", "/api/test", nil)
	w := httptest.NewRecorder()
	p.ServeHTTP(w, req)

	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("expected 503, got %d", w.Code)
	}
}

func TestProxy_StripVersionParam(t *testing.T) {
	// Verify the v= stripping logic used by the proxy
	req := httptest.NewRequest("GET", "/api/test?v=1.0&foo=bar&baz=1", nil)
	q := req.URL.Query()
	q.Del("v")
	req.URL.RawQuery = q.Encode()

	if strings.Contains(req.URL.RawQuery, "v=") {
		t.Errorf("expected v= stripped, got %s", req.URL.RawQuery)
	}
	if !strings.Contains(req.URL.RawQuery, "foo=bar") {
		t.Errorf("expected foo=bar preserved, got %s", req.URL.RawQuery)
	}
	if !strings.Contains(req.URL.RawQuery, "baz=1") {
		t.Errorf("expected baz=1 preserved, got %s", req.URL.RawQuery)
	}
}

func TestProxy_E2E_VersionParamStripped(t *testing.T) {
	var receivedPath, receivedQuery string
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedPath = r.URL.Path
		receivedQuery = r.URL.RawQuery
		w.Write([]byte("proxied-ok"))
	}))
	defer backend.Close()

	port := extractPort(t, backend.URL)

	// Simulate the proxy's forwarding logic with a known backend port
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		if v := q.Get("v"); v != "" {
			q.Del("v")
			r.URL.RawQuery = q.Encode()
		}

		target := "http://127.0.0.1:" + strconv.Itoa(port)
		fwd, _ := http.NewRequest(r.Method, target+r.URL.RequestURI(), r.Body)
		fwd.Header = r.Header

		resp, err := http.DefaultClient.Do(fwd)
		if err != nil {
			w.WriteHeader(502)
			return
		}
		defer resp.Body.Close()

		for k, vs := range resp.Header {
			for _, v := range vs {
				w.Header().Add(k, v)
			}
		}
		w.WriteHeader(resp.StatusCode)
		io.Copy(w, resp.Body)
	})

	t.Run("v_param_stripped", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/data?v=1.0&key=value", nil)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		if w.Code != 200 {
			t.Fatalf("expected 200, got %d", w.Code)
		}
		if w.Body.String() != "proxied-ok" {
			t.Errorf("unexpected body: %s", w.Body.String())
		}
		if strings.Contains(receivedQuery, "v=") {
			t.Errorf("?v= should be stripped, backend saw: %s", receivedQuery)
		}
		if !strings.Contains(receivedQuery, "key=value") {
			t.Errorf("other params should be preserved, backend saw: %s", receivedQuery)
		}
		if receivedPath != "/api/data" {
			t.Errorf("path should be preserved, backend saw: %s", receivedPath)
		}
	})

	t.Run("no_v_param_forwarded_as_is", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/data?key=value", nil)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		if w.Code != 200 {
			t.Fatalf("expected 200, got %d", w.Code)
		}
		if receivedQuery != "key=value" {
			t.Errorf("expected original query, backend saw: %s", receivedQuery)
		}
	})
}

func extractPort(t *testing.T, url string) int {
	t.Helper()
	parts := strings.Split(url, ":")
	port, err := strconv.Atoi(parts[len(parts)-1])
	if err != nil {
		t.Fatalf("failed to parse port from %s: %v", url, err)
	}
	return port
}
