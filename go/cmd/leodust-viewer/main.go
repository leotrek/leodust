package main

import (
	"flag"

	"github.com/leotrek/leodust/pkg/logging"
)

func main() {
	snapshotPath := flag.String(
		"snapshot",
		"./simulation_state_output.gob.json",
		"Path to the default serialized simulation snapshot JSON file",
	)
	snapshotDir := flag.String(
		"snapshotDir",
		"./results/precomputed",
		"Directory of serialized simulation snapshot JSON files to list in the viewer",
	)
	earthImagePath := flag.String(
		"earthImage",
		"./resources/image/World_Map_Blank.svg",
		"Path to the equirectangular world map image used for the Earth globe",
	)
	exportStaticDir := flag.String(
		"exportStaticDir",
		"",
		"Optional output directory for a static GitHub Pages compatible viewer bundle",
	)
	addr := flag.String(
		"addr",
		":8080",
		"HTTP listen address for the viewer",
	)
	logLevel := flag.String(
		"logLevel",
		"info",
		"Log level for the viewer: error, warn, info, debug",
	)
	flag.Parse()

	if err := logging.Configure(*logLevel); err != nil {
		logging.Fatalf("Failed to configure log level: %v", err)
	}

	if *exportStaticDir != "" {
		catalog, err := loadSnapshotCatalog(*snapshotPath, *snapshotDir)
		if err != nil {
			logging.Fatalf("Failed to load snapshots for export: %v", err)
		}
		if err := ExportStaticViewer(*exportStaticDir, catalog, *earthImagePath); err != nil {
			logging.Fatalf("Failed to export static viewer bundle: %v", err)
		}
		logging.Infof("Exported static viewer bundle to %s", *exportStaticDir)
		return
	}

	server, err := NewViewerServer(*snapshotPath, *snapshotDir, *earthImagePath)
	if err != nil {
		logging.Fatalf("Failed to start viewer: %v", err)
	}

	logging.Infof("Serving snapshot viewer on http://localhost%s", *addr)
	logging.Infof("Using snapshot file %s", server.SnapshotPath)
	if server.SnapshotDir != "" {
		logging.Infof("Listing snapshots from %s", server.SnapshotDir)
	}
	logging.Infof("Using Earth image %s", server.EarthImagePath)
	logging.Fatalf("Viewer server stopped: %v", server.ListenAndServe(*addr))
}
