package main

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type snapshotCatalog struct {
	selectedID string
	entries    []snapshotCatalogEntry
	byID       map[string]snapshotCatalogEntry
}

type snapshotCatalogEntry struct {
	ID             string `json:"ID"`
	Name           string `json:"Name"`
	Filename       string `json:"Filename"`
	Path           string `json:"Path,omitempty"`
	SatelliteCount int    `json:"SatelliteCount"`
	GroundCount    int    `json:"GroundCount"`
	FrameCount     int    `json:"FrameCount"`
	path           string
}

type snapshotCatalogResponse struct {
	SelectedID string                 `json:"SelectedID"`
	Snapshots  []snapshotCatalogEntry `json:"Snapshots"`
}

type snapshotSummary struct {
	Satellites []struct{} `json:"Satellites"`
	Grounds    []struct{} `json:"Grounds"`
	States     []struct{} `json:"States"`
}

const (
	viewerConfigFilename      = "viewer-config.json"
	viewerSnapshotCatalogPath = "./snapshots/index.json"
	viewerSnapshotAssetDir    = "snapshots"
	viewerEarthAssetDir       = "earth"
)

type viewerConfig struct {
	EarthImagePath      string `json:"EarthImagePath"`
	SnapshotCatalogPath string `json:"SnapshotCatalogPath"`
}

func loadSnapshotCatalog(snapshotPath, snapshotDir string) (*snapshotCatalog, error) {
	catalog := &snapshotCatalog{
		byID: make(map[string]snapshotCatalogEntry),
	}

	if snapshotDir != "" {
		if err := addSnapshotDirectory(catalog, snapshotDir); err != nil {
			return nil, err
		}
	}

	if snapshotPath != "" {
		if err := addSnapshotFile(catalog, snapshotPath); err != nil {
			if !(errors.Is(err, os.ErrNotExist) && len(catalog.entries) > 0) {
				return nil, err
			}
		}
	}

	if len(catalog.entries) == 0 {
		return nil, errors.New("no snapshot files found")
	}

	sort.Slice(catalog.entries, func(i, j int) bool {
		left := catalog.entries[i]
		right := catalog.entries[j]
		if left.SatelliteCount != right.SatelliteCount {
			return left.SatelliteCount < right.SatelliteCount
		}
		return left.Name < right.Name
	})

	if snapshotPath != "" {
		absoluteSnapshotPath, err := filepath.Abs(snapshotPath)
		if err != nil {
			return nil, err
		}
		for _, entry := range catalog.entries {
			if entry.path == absoluteSnapshotPath {
				catalog.selectedID = entry.ID
				break
			}
		}
	}

	if catalog.selectedID == "" {
		catalog.selectedID = catalog.entries[0].ID
	}

	for _, entry := range catalog.entries {
		catalog.byID[entry.ID] = entry
	}

	return catalog, nil
}

func addSnapshotDirectory(catalog *snapshotCatalog, snapshotDir string) error {
	absoluteDir, err := filepath.Abs(snapshotDir)
	if err != nil {
		return err
	}

	entries, err := os.ReadDir(absoluteDir)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return err
	}

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".gob.json") {
			continue
		}
		if err := addSnapshotFile(catalog, filepath.Join(absoluteDir, entry.Name())); err != nil {
			return err
		}
	}

	return nil
}

func addSnapshotFile(catalog *snapshotCatalog, snapshotPath string) error {
	absoluteSnapshotPath, err := filepath.Abs(snapshotPath)
	if err != nil {
		return err
	}
	if _, err := os.Stat(absoluteSnapshotPath); err != nil {
		return err
	}

	for _, entry := range catalog.entries {
		if entry.path == absoluteSnapshotPath {
			return nil
		}
	}

	summary, err := summarizeSnapshotFile(absoluteSnapshotPath)
	if err != nil {
		return err
	}

	filename := filepath.Base(absoluteSnapshotPath)
	name := strings.TrimSuffix(filename, ".gob.json")
	catalog.entries = append(catalog.entries, snapshotCatalogEntry{
		ID:             name,
		Name:           name,
		Filename:       filename,
		SatelliteCount: len(summary.Satellites),
		GroundCount:    len(summary.Grounds),
		FrameCount:     len(summary.States),
		path:           absoluteSnapshotPath,
	})
	return nil
}

func summarizeSnapshotFile(snapshotPath string) (*snapshotSummary, error) {
	file, err := os.Open(snapshotPath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var summary snapshotSummary
	if err := json.NewDecoder(file).Decode(&summary); err != nil {
		return nil, err
	}
	return &summary, nil
}

func buildViewerConfig(earthImagePath string) viewerConfig {
	return viewerConfig{
		EarthImagePath:      "./" + filepath.ToSlash(filepath.Join(viewerEarthAssetDir, filepath.Base(earthImagePath))),
		SnapshotCatalogPath: viewerSnapshotCatalogPath,
	}
}

func buildSnapshotCatalogResponse(catalog *snapshotCatalog) snapshotCatalogResponse {
	entries := make([]snapshotCatalogEntry, 0, len(catalog.entries))
	for _, entry := range catalog.entries {
		manifestEntry := entry
		manifestEntry.Path = "./" + filepath.ToSlash(filepath.Join(viewerSnapshotAssetDir, manifestEntry.Filename))
		manifestEntry.path = ""
		entries = append(entries, manifestEntry)
	}

	return snapshotCatalogResponse{
		SelectedID: catalog.selectedID,
		Snapshots:  entries,
	}
}

func lookupSnapshotEntryByFilename(catalog *snapshotCatalog, filename string) (snapshotCatalogEntry, bool) {
	for _, entry := range catalog.entries {
		if entry.Filename == filename {
			return entry, true
		}
	}
	return snapshotCatalogEntry{}, false
}
