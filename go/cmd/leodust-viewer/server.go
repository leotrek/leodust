package main

import (
	"embed"
	"encoding/json"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/leotrek/leodust/pkg/logging"
)

//go:embed static/*
var staticFiles embed.FS

// ViewerServer serves the snapshot API and static viewer assets.
type ViewerServer struct {
	SnapshotPath   string
	SnapshotDir    string
	EarthImagePath string
	handler        http.Handler
	catalog        *snapshotCatalog
}

// NewViewerServer validates the snapshot file and prepares the HTTP handlers.
func NewViewerServer(snapshotPath, snapshotDir, earthImagePath string) (*ViewerServer, error) {
	absoluteEarthImagePath, err := filepath.Abs(earthImagePath)
	if err != nil {
		return nil, err
	}
	if _, err := os.Stat(absoluteEarthImagePath); err != nil {
		return nil, err
	}

	staticRoot, err := fs.Sub(staticFiles, "static")
	if err != nil {
		return nil, err
	}

	catalog, err := loadSnapshotCatalog(snapshotPath, snapshotDir)
	if err != nil {
		return nil, err
	}

	selected, ok := catalog.byID[catalog.selectedID]
	if !ok {
		return nil, os.ErrNotExist
	}

	absoluteSnapshotDir := ""
	if snapshotDir != "" {
		absoluteSnapshotDir, err = filepath.Abs(snapshotDir)
		if err != nil {
			return nil, err
		}
	}

	server := &ViewerServer{
		SnapshotPath:   selected.path,
		SnapshotDir:    absoluteSnapshotDir,
		EarthImagePath: absoluteEarthImagePath,
		catalog:        catalog,
	}
	server.handler = buildViewerHandler(server, staticRoot)
	return server, nil
}

// ListenAndServe starts the viewer HTTP server.
func (s *ViewerServer) ListenAndServe(addr string) error {
	return http.ListenAndServe(addr, s.handler)
}

func buildViewerHandler(server *ViewerServer, staticRoot fs.FS) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/"+viewerConfigFilename, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		_ = json.NewEncoder(w).Encode(buildViewerConfig(server.EarthImagePath))
	})
	mux.HandleFunc("/"+viewerSnapshotAssetDir+"/index.json", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		_ = json.NewEncoder(w).Encode(buildSnapshotCatalogResponse(server.catalog))
	})
	mux.HandleFunc("/"+viewerSnapshotAssetDir+"/", func(w http.ResponseWriter, r *http.Request) {
		filename := filepath.Base(strings.TrimPrefix(r.URL.Path, "/"+viewerSnapshotAssetDir+"/"))
		entry, ok := lookupSnapshotEntryByFilename(server.catalog, filename)
		if !ok {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		http.ServeFile(w, r, entry.path)
	})
	mux.HandleFunc("/"+viewerEarthAssetDir+"/", func(w http.ResponseWriter, r *http.Request) {
		if filepath.Base(server.EarthImagePath) != filepath.Base(strings.TrimPrefix(r.URL.Path, "/"+viewerEarthAssetDir+"/")) {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Cache-Control", "no-store")
		http.ServeFile(w, r, server.EarthImagePath)
	})

	fileServer := http.FileServer(http.FS(staticRoot))
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" {
			http.ServeFileFS(w, r, staticRoot, "index.html")
			return
		}

		fileServer.ServeHTTP(w, r)
	})

	return requestLogger(mux)
}

func requestLogger(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		logging.Debugf("viewer request %s %s", r.Method, r.URL.Path)
		next.ServeHTTP(w, r)
	})
}
