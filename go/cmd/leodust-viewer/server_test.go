package main

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNewViewerServerRequiresSnapshot(t *testing.T) {
	if _, err := NewViewerServer(filepath.Join(t.TempDir(), "missing.json"), "", writeEarthImageFixture(t)); err == nil {
		t.Fatal("expected missing snapshot file to return an error")
	}
}

func TestViewerServerSnapshotEndpoint(t *testing.T) {
	snapshotPath := writeSnapshotFixture(t, `{"States":[{"Time":"2026-03-26T16:00:00Z","NodeStates":[]}]}`)
	server, err := NewViewerServer(snapshotPath, "", writeEarthImageFixture(t))
	if err != nil {
		t.Fatalf("NewViewerServer returned error: %v", err)
	}

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/snapshots/snapshot.json", nil)
	server.handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", recorder.Code)
	}
	if contentType := recorder.Header().Get("Content-Type"); !strings.Contains(contentType, "application/json") {
		t.Fatalf("expected JSON content type, got %q", contentType)
	}
	if !strings.Contains(recorder.Body.String(), `"States"`) {
		t.Fatalf("unexpected snapshot body: %s", recorder.Body.String())
	}
}

func TestViewerServerIndex(t *testing.T) {
	snapshotPath := writeSnapshotFixture(t, `{"States":[]}`)
	server, err := NewViewerServer(snapshotPath, "", writeEarthImageFixture(t))
	if err != nil {
		t.Fatalf("NewViewerServer returned error: %v", err)
	}

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/", nil)
	server.handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", recorder.Code)
	}
	if !strings.Contains(recorder.Body.String(), "LeoDust Viewer") {
		t.Fatalf("expected index HTML, got %s", recorder.Body.String())
	}
}

func TestViewerServerEarthImageEndpoint(t *testing.T) {
	snapshotPath := writeSnapshotFixture(t, `{"States":[]}`)
	earthImagePath := writeEarthImageFixture(t)
	server, err := NewViewerServer(snapshotPath, "", earthImagePath)
	if err != nil {
		t.Fatalf("NewViewerServer returned error: %v", err)
	}

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/earth/"+filepath.Base(earthImagePath), nil)
	server.handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", recorder.Code)
	}
	if !strings.Contains(recorder.Body.String(), "<svg") {
		t.Fatalf("expected SVG body, got %s", recorder.Body.String())
	}
}

func TestViewerServerSnapshotCatalogEndpoint(t *testing.T) {
	snapshotDir := t.TempDir()
	writeSnapshotFixtureAt(t, snapshotDir, "precomputed_data-0250.gob.json", `{"States":[{}],"Satellites":[{},{}],"Grounds":[{}]}`)
	writeSnapshotFixtureAt(t, snapshotDir, "precomputed_data-0500.gob.json", `{"States":[{},{}],"Satellites":[{},{},{}],"Grounds":[{},{}]}`)

	server, err := NewViewerServer("", snapshotDir, writeEarthImageFixture(t))
	if err != nil {
		t.Fatalf("NewViewerServer returned error: %v", err)
	}

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/snapshots/index.json", nil)
	server.handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", recorder.Code)
	}
	body := recorder.Body.String()
	if !strings.Contains(body, `"SatelliteCount":2`) {
		t.Fatalf("expected 2-satellite entry, got %s", body)
	}
	if !strings.Contains(body, `"SatelliteCount":3`) {
		t.Fatalf("expected 3-satellite entry, got %s", body)
	}
	if !strings.Contains(body, `"Path":"./snapshots/precomputed_data-0250.gob.json"`) {
		t.Fatalf("expected catalog path entry, got %s", body)
	}
}

func TestNewViewerServerFallsBackToSnapshotDirectoryWhenDefaultSnapshotIsMissing(t *testing.T) {
	snapshotDir := t.TempDir()
	writeSnapshotFixtureAt(t, snapshotDir, "precomputed_data-0250.gob.json", `{"States":[{}],"Satellites":[{},{}],"Grounds":[{}]}`)

	server, err := NewViewerServer(filepath.Join(t.TempDir(), "missing.json"), snapshotDir, writeEarthImageFixture(t))
	if err != nil {
		t.Fatalf("NewViewerServer returned error: %v", err)
	}

	if !strings.HasSuffix(server.SnapshotPath, "precomputed_data-0250.gob.json") {
		t.Fatalf("expected server to select catalog snapshot, got %q", server.SnapshotPath)
	}
}

func TestViewerServerConfigEndpoint(t *testing.T) {
	snapshotPath := writeSnapshotFixture(t, `{"States":[]}`)
	earthImagePath := writeEarthImageFixture(t)
	server, err := NewViewerServer(snapshotPath, "", earthImagePath)
	if err != nil {
		t.Fatalf("NewViewerServer returned error: %v", err)
	}

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/viewer-config.json", nil)
	server.handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", recorder.Code)
	}
	body := recorder.Body.String()
	if !strings.Contains(body, `"SnapshotCatalogPath":"./snapshots/index.json"`) {
		t.Fatalf("expected snapshot catalog path in config, got %s", body)
	}
	if !strings.Contains(body, `"EarthImagePath":"./earth/earth.svg"`) {
		t.Fatalf("expected earth image path in config, got %s", body)
	}
}

func TestExportStaticViewerWritesGitHubPagesBundle(t *testing.T) {
	snapshotDir := t.TempDir()
	writeSnapshotFixtureAt(t, snapshotDir, "precomputed_data-0250.gob.json", `{"States":[{}],"Satellites":[{},{}],"Grounds":[{}]}`)
	earthImagePath := writeEarthImageFixture(t)

	catalog, err := loadSnapshotCatalog("", snapshotDir)
	if err != nil {
		t.Fatalf("loadSnapshotCatalog returned error: %v", err)
	}

	outputDir := filepath.Join(t.TempDir(), "docs")
	if err := ExportStaticViewer(outputDir, catalog, earthImagePath); err != nil {
		t.Fatalf("ExportStaticViewer returned error: %v", err)
	}

	assertFileExists(t, filepath.Join(outputDir, "index.html"))
	assertFileExists(t, filepath.Join(outputDir, "app.js"))
	assertFileExists(t, filepath.Join(outputDir, "styles.css"))
	assertFileExists(t, filepath.Join(outputDir, "viewer-config.json"))
	assertFileExists(t, filepath.Join(outputDir, "snapshots", "index.json"))
	assertFileExists(t, filepath.Join(outputDir, "snapshots", "precomputed_data-0250.gob.json"))
	assertFileExists(t, filepath.Join(outputDir, "earth", filepath.Base(earthImagePath)))
}

func writeSnapshotFixture(t *testing.T, content string) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), "snapshot.json")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("failed to write snapshot fixture: %v", err)
	}
	return path
}

func writeSnapshotFixtureAt(t *testing.T, dir, name, content string) string {
	t.Helper()

	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("failed to write snapshot fixture: %v", err)
	}
	return path
}

func writeEarthImageFixture(t *testing.T) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), "earth.svg")
	content := `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 2 1"><rect width="2" height="1" fill="#1b4f72"/><circle cx="0.7" cy="0.5" r="0.2" fill="#a3b96f"/></svg>`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("failed to write earth image fixture: %v", err)
	}
	return path
}

func assertFileExists(t *testing.T, path string) {
	t.Helper()

	if _, err := os.Stat(path); err != nil {
		t.Fatalf("expected file %s to exist: %v", path, err)
	}
}
