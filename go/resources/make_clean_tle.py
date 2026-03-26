#!/usr/bin/env python3
import argparse
from collections import defaultdict


def parse_tle(path):
    with open(path, "r", encoding="utf-8") as f:
        lines = [line.rstrip("\n") for line in f if line.strip()]

    if len(lines) % 3 != 0:
        raise ValueError(f"{path}: expected 3 lines per satellite, got {len(lines)} lines")

    sats = []
    for i in range(0, len(lines), 3):
        name = lines[i].strip()
        l1 = lines[i + 1].strip()
        l2 = lines[i + 2].strip()

        if not l1.startswith("1 ") or not l2.startswith("2 "):
            raise ValueError(f"Invalid TLE block at line {i+1}")

        norad = l1[2:7].strip()
        inclination = float(l2[8:16].strip())
        raan = float(l2[17:25].strip())

        sats.append({
            "name": name,
            "norad": norad,
            "inclination": inclination,
            "raan": raan,
            "lines": [lines[i] + "\n", lines[i + 1] + "\n", lines[i + 2] + "\n"],
        })
    return sats


def dedup_by_name(sats):
    out = []
    seen = set()
    for s in sats:
        if s["name"] in seen:
            continue
        seen.add(s["name"])
        out.append(s)
    return out


def filter_inclination(sats, center, tol):
    return [s for s in sats if abs(s["inclination"] - center) <= tol]


def sample_even_raan(sats, target, bin_size=5):
    bins = defaultdict(list)
    for s in sats:
        b = int(s["raan"] // bin_size) * bin_size
        bins[b].append(s)

    ordered_bins = sorted(bins.keys())
    for b in ordered_bins:
        bins[b].sort(key=lambda s: (s["raan"], s["name"]))

    selected = []
    used_names = set()

    # round-robin across RAAN bins
    while len(selected) < target:
        progressed = False
        for b in ordered_bins:
            while bins[b]:
                s = bins[b].pop(0)
                if s["name"] in used_names:
                    continue
                used_names.add(s["name"])
                selected.append(s)
                progressed = True
                break
            if len(selected) >= target:
                break
        if not progressed:
            break

    return selected


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("--in", dest="input_path", required=True)
    ap.add_argument("--out", dest="output_path", required=True)
    ap.add_argument("--target", type=int, required=True)
    ap.add_argument("--inclination", type=float, default=53.0)
    ap.add_argument("--tol", type=float, default=1.0)
    ap.add_argument("--raan-bin", type=int, default=5)
    args = ap.parse_args()

    sats = parse_tle(args.input_path)
    sats = dedup_by_name(sats)
    sats = filter_inclination(sats, args.inclination, args.tol)

    if len(sats) < args.target:
        raise SystemExit(
            f"Only {len(sats)} satellites after filtering around inclination "
            f"{args.inclination}±{args.tol}, need {args.target}"
        )

    selected = sample_even_raan(sats, args.target, args.raan_bin)

    if len(selected) < args.target:
        raise SystemExit(f"Could only select {len(selected)} satellites, need {args.target}")

    with open(args.output_path, "w", encoding="utf-8") as f:
        for s in selected:
            f.writelines(s["lines"])

    print("Wrote:", args.output_path)
    print("Selected satellites:", len(selected))
    print("Inclination filter:", args.inclination, "±", args.tol)
    print("RAAN bin size:", args.raan_bin)


if __name__ == "__main__":
    main()
