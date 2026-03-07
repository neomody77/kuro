// Package ui provides the minimal recovery admin UI.
package ui

import (
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/neomody77/kuro/recovery/health"
	"github.com/neomody77/kuro/recovery/version"
)

// Handler serves the recovery admin UI and API.
type Handler struct {
	manager *version.Manager
	checker *health.Checker
	mux     *http.ServeMux
}

// New creates a new admin UI handler.
func New(manager *version.Manager, checker *health.Checker) *Handler {
	h := &Handler{
		manager: manager,
		checker: checker,
		mux:     http.NewServeMux(),
	}
	h.mux.HandleFunc("/api/versions", h.handleVersions)
	h.mux.HandleFunc("/api/versions/upload", h.handleUpload)
	h.mux.HandleFunc("/api/versions/", h.handleVersionAction)
	h.mux.HandleFunc("/api/health", h.handleHealth)
	h.mux.HandleFunc("/", h.handleIndex)
	return h
}

// ServeHTTP implements http.Handler.
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	h.mux.ServeHTTP(w, r)
}

type versionInfo struct {
	ID       string `json:"id"`
	Port     int    `json:"port"`
	Status   string `json:"status"`
	Current  bool   `json:"current"`
	Failures int    `json:"failures"`
}

func (h *Handler) handleVersions(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	versions := h.manager.List()
	currentID := h.manager.Current()

	infos := make([]versionInfo, len(versions))
	for i, v := range versions {
		infos[i] = versionInfo{
			ID:       v.ID,
			Port:     v.Port,
			Status:   string(v.Status),
			Current:  v.ID == currentID,
			Failures: h.checker.Failures(v.ID),
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"versions":     infos,
		"current":      currentID,
		"autoRollback": h.checker.AutoRollback(),
	})
}

func (h *Handler) handleVersionAction(w http.ResponseWriter, r *http.Request) {
	// Parse: /api/versions/{id}/{action}
	path := strings.TrimPrefix(r.URL.Path, "/api/versions/")
	parts := strings.SplitN(path, "/", 2)
	if len(parts) < 1 || parts[0] == "" {
		http.Error(w, "version id required", http.StatusBadRequest)
		return
	}

	id := parts[0]
	action := ""
	if len(parts) == 2 {
		action = parts[1]
	}

	switch {
	case r.Method == http.MethodPost && action == "start":
		if err := h.manager.Start(id); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		h.jsonOK(w, "started")

	case r.Method == http.MethodPost && action == "stop":
		if err := h.manager.Stop(id); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		h.jsonOK(w, "stopped")

	case r.Method == http.MethodPost && action == "default":
		if err := h.manager.SetDefault(id); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		h.jsonOK(w, "default set")

	case r.Method == http.MethodDelete && action == "":
		if err := h.manager.Delete(id); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		h.jsonOK(w, "deleted")

	default:
		http.Error(w, "not found", http.StatusNotFound)
	}
}

func (h *Handler) handleUpload(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if err := r.ParseMultipartForm(256 << 20); err != nil { // 256MB max
		http.Error(w, "failed to parse form: "+err.Error(), http.StatusBadRequest)
		return
	}

	versionID := r.FormValue("version")
	if versionID == "" {
		http.Error(w, "version field required", http.StatusBadRequest)
		return
	}

	// Save binary to temp file
	binaryFile, _, err := r.FormFile("binary")
	if err != nil {
		http.Error(w, "binary file required: "+err.Error(), http.StatusBadRequest)
		return
	}
	defer binaryFile.Close()

	tmpDir, err := os.MkdirTemp("", "kuro-upload-*")
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	defer os.RemoveAll(tmpDir)

	binaryPath := filepath.Join(tmpDir, "kuro")
	bf, err := os.Create(binaryPath)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	io.Copy(bf, binaryFile)
	bf.Close()

	// Optional UI file
	var uiPath string
	uiFile, uiHeader, err := r.FormFile("ui")
	if err == nil {
		defer uiFile.Close()
		uiPath = filepath.Join(tmpDir, uiHeader.Filename)
		uf, err := os.Create(uiPath)
		if err != nil {
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		io.Copy(uf, uiFile)
		uf.Close()
	}

	if err := h.manager.Install(binaryPath, uiPath, versionID); err != nil {
		http.Error(w, "install failed: "+err.Error(), http.StatusInternalServerError)
		return
	}

	h.jsonOK(w, "installed")
}

func (h *Handler) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

func (h *Handler) jsonOK(w http.ResponseWriter, msg string) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": msg})
}

func (h *Handler) handleIndex(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}

	versions := h.manager.List()
	currentID := h.manager.Current()

	type tmplVersion struct {
		ID       string
		Port     int
		Status   string
		Current  bool
		Failures int
	}

	data := struct {
		Versions     []tmplVersion
		Current      string
		AutoRollback bool
	}{
		Current:      currentID,
		AutoRollback: h.checker.AutoRollback(),
	}
	for _, v := range versions {
		data.Versions = append(data.Versions, tmplVersion{
			ID:       v.ID,
			Port:     v.Port,
			Status:   string(v.Status),
			Current:  v.ID == currentID,
			Failures: h.checker.Failures(v.ID),
		})
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := adminTmpl.Execute(w, data); err != nil {
		fmt.Fprintf(w, "template error: %v", err)
	}
}

var adminTmpl = template.Must(template.New("admin").Parse(`<!DOCTYPE html>
<html>
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>Kuro Recovery</title>
<style>
  * { box-sizing: border-box; margin: 0; padding: 0; }
  body { font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", sans-serif; background: #0f0f0f; color: #e0e0e0; padding: 2rem; }
  h1 { margin-bottom: 1.5rem; color: #fff; }
  .info { color: #888; margin-bottom: 1rem; font-size: 0.9rem; }
  table { width: 100%; border-collapse: collapse; margin-bottom: 2rem; }
  th, td { padding: 0.75rem 1rem; text-align: left; border-bottom: 1px solid #2a2a2a; }
  th { color: #888; font-weight: 500; font-size: 0.85rem; text-transform: uppercase; }
  .running { color: #4ade80; }
  .stopped { color: #888; }
  .current-badge { background: #3b82f6; color: #fff; padding: 0.15rem 0.5rem; border-radius: 3px; font-size: 0.8rem; margin-left: 0.5rem; }
  button { padding: 0.35rem 0.75rem; border: 1px solid #333; background: #1a1a1a; color: #e0e0e0; border-radius: 4px; cursor: pointer; margin-right: 0.25rem; font-size: 0.85rem; }
  button:hover { background: #2a2a2a; }
  button.danger { border-color: #7f1d1d; color: #f87171; }
  button.danger:hover { background: #7f1d1d; color: #fff; }
  button.primary { border-color: #1e40af; color: #60a5fa; }
  button.primary:hover { background: #1e40af; color: #fff; }
  .upload { border: 1px solid #2a2a2a; border-radius: 6px; padding: 1.5rem; margin-top: 1rem; }
  .upload h2 { font-size: 1rem; margin-bottom: 1rem; }
  .upload label { display: block; margin-bottom: 0.5rem; color: #888; font-size: 0.85rem; }
  .upload input[type="text"], .upload input[type="file"] { margin-bottom: 0.75rem; }
  #status { margin-top: 1rem; padding: 0.5rem; font-size: 0.9rem; }
</style>
</head>
<body>
<h1>Kuro Recovery</h1>
<p class="info">Current: <strong>{{.Current}}</strong> | Auto-rollback: {{if .AutoRollback}}on{{else}}off{{end}}</p>

<table>
  <thead><tr><th>Version</th><th>Status</th><th>Port</th><th>Failures</th><th>Actions</th></tr></thead>
  <tbody>
  {{range .Versions}}
  <tr>
    <td>{{.ID}}{{if .Current}}<span class="current-badge">current</span>{{end}}</td>
    <td class="{{.Status}}">{{.Status}}</td>
    <td>{{if eq .Port 0}}-{{else}}{{.Port}}{{end}}</td>
    <td>{{.Failures}}</td>
    <td>
      {{if eq .Status "stopped"}}
        <button onclick="api('/api/versions/{{.ID}}/start','POST')">Start</button>
      {{else}}
        <button onclick="api('/api/versions/{{.ID}}/stop','POST')">Stop</button>
      {{end}}
      {{if not .Current}}
        <button class="primary" onclick="api('/api/versions/{{.ID}}/default','POST')">Set Default</button>
        <button class="danger" onclick="if(confirm('Delete {{.ID}}?'))api('/api/versions/{{.ID}}','DELETE')">Delete</button>
      {{end}}
    </td>
  </tr>
  {{else}}
  <tr><td colspan="5" style="color:#888">No versions installed</td></tr>
  {{end}}
  </tbody>
</table>

<div class="upload">
  <h2>Upload New Version</h2>
  <form id="uploadForm" enctype="multipart/form-data">
    <label>Version ID</label>
    <input type="text" name="version" placeholder="e.g. 0.3.0" required>
    <label>Engine Binary</label>
    <input type="file" name="binary" required>
    <label>UI Assets (optional)</label>
    <input type="file" name="ui">
    <button type="submit" class="primary">Upload</button>
  </form>
</div>

<div id="status"></div>

<script>
async function api(url, method) {
  try {
    const r = await fetch(url, {method});
    const t = await r.text();
    document.getElementById('status').textContent = r.ok ? 'OK: '+t : 'Error: '+t;
    if(r.ok) setTimeout(()=>location.reload(), 500);
  } catch(e) {
    document.getElementById('status').textContent = 'Error: '+e.message;
  }
}
document.getElementById('uploadForm').onsubmit = async function(e) {
  e.preventDefault();
  const fd = new FormData(this);
  try {
    const r = await fetch('/api/versions/upload', {method:'POST', body:fd});
    const t = await r.text();
    document.getElementById('status').textContent = r.ok ? 'Uploaded: '+t : 'Error: '+t;
    if(r.ok) setTimeout(()=>location.reload(), 500);
  } catch(e) {
    document.getElementById('status').textContent = 'Error: '+e.message;
  }
};
</script>
</body>
</html>`))
