package main

import (
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
)

// ExportStaticViewer writes a self-contained snapshot viewer bundle that can be
// hosted on a static site provider such as GitHub Pages.
func ExportStaticViewer(outputDir string, catalog *snapshotCatalog, earthImagePath string) error {
	staticRoot, err := fs.Sub(staticFiles, "static")
	if err != nil {
		return err
	}

	absoluteOutputDir, err := filepath.Abs(outputDir)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(absoluteOutputDir, 0o755); err != nil {
		return err
	}

	if err := copyEmbeddedStaticFiles(staticRoot, absoluteOutputDir); err != nil {
		return err
	}
	if err := writeViewerConfigFile(absoluteOutputDir, buildViewerConfig(earthImagePath)); err != nil {
		return err
	}
	if err := writeSnapshotCatalogFile(absoluteOutputDir, buildSnapshotCatalogResponse(catalog)); err != nil {
		return err
	}
	if err := copySnapshotFiles(absoluteOutputDir, catalog); err != nil {
		return err
	}
	if err := copyEarthImageFile(absoluteOutputDir, earthImagePath); err != nil {
		return err
	}

	return nil
}

func copyEmbeddedStaticFiles(staticRoot fs.FS, outputDir string) error {
	return fs.WalkDir(staticRoot, ".", func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			if path == "." {
				return nil
			}
			return os.MkdirAll(filepath.Join(outputDir, path), 0o755)
		}

		contents, err := fs.ReadFile(staticRoot, path)
		if err != nil {
			return err
		}
		targetPath := filepath.Join(outputDir, path)
		if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
			return err
		}
		return os.WriteFile(targetPath, contents, 0o644)
	})
}

func writeViewerConfigFile(outputDir string, config viewerConfig) error {
	targetPath := filepath.Join(outputDir, viewerConfigFilename)
	return writeJSONFile(targetPath, config)
}

func writeSnapshotCatalogFile(outputDir string, response snapshotCatalogResponse) error {
	targetPath := filepath.Join(outputDir, filepath.FromSlash(viewerSnapshotCatalogPath))
	return writeJSONFile(targetPath, response)
}

func copySnapshotFiles(outputDir string, catalog *snapshotCatalog) error {
	targetDir := filepath.Join(outputDir, viewerSnapshotAssetDir)
	if err := os.MkdirAll(targetDir, 0o755); err != nil {
		return err
	}

	for _, entry := range catalog.entries {
		targetPath := filepath.Join(targetDir, entry.Filename)
		if err := copyFile(entry.path, targetPath); err != nil {
			return fmt.Errorf("copy snapshot %s: %w", entry.Filename, err)
		}
	}
	return nil
}

func copyEarthImageFile(outputDir, earthImagePath string) error {
	targetDir := filepath.Join(outputDir, viewerEarthAssetDir)
	if err := os.MkdirAll(targetDir, 0o755); err != nil {
		return err
	}

	targetPath := filepath.Join(targetDir, filepath.Base(earthImagePath))
	return copyFile(earthImagePath, targetPath)
}

func writeJSONFile(targetPath string, value any) error {
	if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
		return err
	}

	contents, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	contents = append(contents, '\n')
	return os.WriteFile(targetPath, contents, 0o644)
}

func copyFile(sourcePath, targetPath string) error {
	source, err := os.Open(sourcePath)
	if err != nil {
		return err
	}
	defer source.Close()

	if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
		return err
	}

	target, err := os.Create(targetPath)
	if err != nil {
		return err
	}

	if _, err := io.Copy(target, source); err != nil {
		_ = target.Close()
		return err
	}
	return target.Close()
}
