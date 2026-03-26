# Viewer and GitHub Pages

This tutorial shows how to use the snapshot viewer locally and how to export it as a static site for GitHub Pages.

## What the Viewer Expects

The viewer is snapshot-based. It reads precomputed JSON snapshots and renders:

- Earth
- satellites
- ground stations
- active links
- timeline playback
- dataset selection

## Step 1: Start the Viewer Locally

From `go/`:

```bash
go run ./cmd/leodust-viewer \
  --snapshotDir ./results/precomputed \
  --earthImage ./resources/image/World_Map_Blank.svg \
  --addr :8080
```

Then open:

```text
http://localhost:8080
```

## Step 2: Understand the Viewer Inputs

The viewer consumes:

- snapshot JSON files in `./results/precomputed`
- an equirectangular Earth map image

The current UI already includes:

- a dataset dropdown populated from the snapshot directory
- play/pause and timeline controls
- filters for satellites, ground stations, and links
- a satellite render limit
- node selection

## Step 3: Understand the Static Export

The viewer can be exported as a fully static site:

```bash
go run ./cmd/leodust-viewer \
  --snapshotDir ./results/precomputed \
  --earthImage ./resources/image/World_Map_Blank.svg \
  --exportStaticDir ../viewer-site
```

That writes:

- `index.html`
- `app.js`
- `styles.css`
- `viewer-config.json`
- `snapshots/index.json`
- `snapshots/*.gob.json`
- `earth/<image>`

## Step 4: Why Static Export Works

The viewer was refactored to use relative static files instead of requiring Go-only API endpoints.

That means the same UI can run:

- through the local server
- from a static host like GitHub Pages

## Step 5: Keep the Viewer Export Separate from MkDocs

If you are using MkDocs or Read the Docs, keep these folders separate:

- `docs/` for MkDocs source
- `site/` for MkDocs build output
- `viewer-site/` or another folder for the snapshot viewer export

Do not export the viewer into `docs/`, because that directory now contains the documentation source.

## Step 6: Publish on GitHub Pages

One simple pattern is to export into a dedicated folder:

```bash
go run ./cmd/leodust-viewer \
  --snapshotDir ./results/precomputed \
  --earthImage ./resources/image/World_Map_Blank.svg \
  --exportStaticDir ../viewer-site
```

Then publish that folder through the host of your choice or a dedicated Pages branch/workflow.

## Step 7: Understand the Viewer Config Files

The static export writes:

### `viewer-config.json`

This tells the browser app:

- where the Earth image is
- where the snapshot catalog is

### `snapshots/index.json`

This is the manifest for the dataset dropdown. It includes:

- snapshot ID
- filename
- relative path
- satellite count
- ground-station count
- frame count

## Common Viewer Questions

### Is the Earth aligned correctly?

Yes, the viewer renders node positions and the Earth image in the same ECEF frame.

### Can the viewer stream live simulation?

Not currently. The current viewer is snapshot-only.

### Can this run on Read the Docs?

No. Read the Docs is for documentation pages. The viewer static bundle is for a normal static host such as GitHub Pages.

## Next Step

Continue to [Changing Configurations](changing-configurations.md).
