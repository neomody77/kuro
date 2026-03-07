// Package proxy implements the reverse proxy with ?v= version routing.
package proxy

import (
	"fmt"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"

	"github.com/neomody77/kuro/recovery/version"
)

// Proxy routes requests to engine versions based on ?v= query parameter.
type Proxy struct {
	manager *version.Manager
}

// New creates a new version-routing reverse proxy.
func New(manager *version.Manager) *Proxy {
	return &Proxy{manager: manager}
}

// ServeHTTP implements http.Handler.
func (p *Proxy) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	versionID := q.Get("v")

	var port int
	if versionID != "" {
		port = p.manager.PortForVersion(versionID)
		if port == 0 {
			http.Error(w, fmt.Sprintf("version %s is not running", versionID), http.StatusServiceUnavailable)
			return
		}
		// Strip ?v= param before forwarding
		q.Del("v")
		r.URL.RawQuery = q.Encode()
	} else {
		port = p.manager.DefaultPort()
		if port == 0 {
			http.Error(w, "no default version is running", http.StatusServiceUnavailable)
			return
		}
	}

	target, err := url.Parse(fmt.Sprintf("http://127.0.0.1:%d", port))
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	proxy := httputil.NewSingleHostReverseProxy(target)
	proxy.ErrorHandler = func(rw http.ResponseWriter, req *http.Request, err error) {
		log.Printf("proxy error to port %d: %v", port, err)
		rw.WriteHeader(http.StatusBadGateway)
		fmt.Fprintf(rw, "engine unavailable: %v", err)
	}

	proxy.ServeHTTP(w, r)
}
