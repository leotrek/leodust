# Data Files

This page explains the file inputs and outputs used by LeoDust.

## TLE Files

Bundled TLE files live in `go/resources/tle`.

### Main Naming Conventions

- `starlink_current.tle`
  Latest bundled full snapshot.
- `starlink_<count>.tle`
  Maintained subset files for repeatable local runs.
- `starlink_6000.tle`
  Large raw subset kept as-is.

### What LeoDust Reads from TLEs

The TLE loader extracts:

- orbital identity/name
- epoch
- orbital elements needed for propagation

The actual propagation is then handled by the SGP4 wrapper in the orbit package.

### Trustworthiness Rule

The most important operational rule is:

- keep `SimulationStartTime` close to the TLE epoch

## Ground-Station YAML Files

Bundled ground stations live in:

- `go/resources/yml/ground_stations.yml`

They define:

- name
- latitude
- longitude
- optional altitude
- per-station ground-link override
- per-station computing override

## Precomputed Simulation Files

When you pass `--simulationStateOutputFile ./path/name.gob`, LeoDust writes:

- `./path/name.gob`
- `./path/name.gob.json`

### `.gob`

Used by replay mode.

### `.json`

Used by the browser viewer and by the static export path.

## Viewer Static Export Files

The viewer export writes:

- `index.html`
- `app.js`
- `styles.css`
- `viewer-config.json`
- `snapshots/index.json`
- `snapshots/*.gob.json`
- `earth/<image>`

### `viewer-config.json`

Defines:

- the Earth image location
- the snapshot manifest location

### `snapshots/index.json`

Defines:

- available datasets
- selected default dataset
- dataset filenames
- dataset relative paths
- satellite counts
- ground-station counts
- frame counts
