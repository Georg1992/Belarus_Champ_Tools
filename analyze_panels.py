"""Compute pixel statistics on a set of true-positive and false-positive panel
crops + the embedded StatusPanel.png template. Output is a single table that
makes the discriminating signals between real status panel UI and skill/hotbar
false-positives visually obvious.

Reads (relative to clicker/):
  - runner/statusui/StatusPanel.png           (the template)
  - runner/statusui/debug_six_named/panel_<prefix>_*.png   (test crops)
  - runner/statusui/debug/panel_<prefix>_*.png              (older debug dump)

For each file that's 218x58, computes:
  blue%       : pixels where B > R+25 && B > G+25 && B > 80   (bluish UI accents)
  red%        : pixels where R > G+30 && R > B+30             (HP bar fill red)
  white%      : pixels where R,G,B all >= 235                 (panel background)
  dark%       : pixels where luminance < 50                   (digits / symbols)
  TL_con / TR_con : per-channel stddev in 12x12 corner patches (decorative circles)
  edge%       : fraction of neighbour-pairs with luminance jump > 80 (has text)
  topred%     : red% restricted to y < 18                     (HP bar band fill)

Usage:
    cd clicker && python ../analyze_panels.py
"""
from PIL import Image
import glob
import os
import statistics


def stats(path, name):
    try:
        img = Image.open(path)
        w, h = img.size
        if (w, h) != (218, 58):
            return {"name": name, "err": f"WRONG DIMS ({w}x{h})"}

        px = img.load()
        blue = red = white = dark = 0
        stripe_red = 0
        stripe_total = 0
        luma = [
            [
                (299 * px[x, y][0] + 587 * px[x, y][1] + 114 * px[x, y][2]) // 1000
                for x in range(w)
            ]
            for y in range(h)
        ]
        edges = 0
        edge_total = 0
        for y in range(2, h - 2):
            for x in range(2, w - 2):
                edge_total += 4
                c = luma[y][x]
                for dx, dy in ((-2, 0), (2, 0), (0, -2), (0, 2)):
                    if abs(c - luma[y + dy][x + dx]) > 80:
                        edges += 1

        for y in range(h):
            for x in range(w):
                r, g, b = px[x, y][:3]
                if b > r + 25 and b > g + 25 and b > 80:
                    blue += 1
                if r > g + 30 and r > b + 30:
                    red += 1
                    if y < 18:
                        stripe_red += 1
                if r >= 235 and g >= 235 and b >= 235:
                    white += 1
                lum = (299 * r + 587 * g + 114 * b) // 1000
                if lum < 50:
                    dark += 1
                if y < 18:
                    stripe_total += 1

        def corner(cx, cy):
            vals = []
            for yy in range(cy, cy + 12):
                for xx in range(cx, cx + 12):
                    vals.append(px[xx, yy][:3])
            return round(
                (
                    statistics.stdev([v[0] for v in vals])
                    + statistics.stdev([v[1] for v in vals])
                    + statistics.stdev([v[2] for v in vals])
                )
                / 3,
                1,
            )

        total = w * h
        return {
            "name": name,
            "blue%": round(100 * blue / total, 2),
            "red%": round(100 * red / total, 2),
            "white%": round(100 * white / total, 2),
            "dark%": round(100 * dark / total, 2),
            "TL_con": corner(0, 0),
            "TR_con": corner(w - 12, 0),
            "edge%": round(100 * edges / max(edge_total, 1), 2),
            "topred%": round(100 * stripe_red / max(stripe_total, 1), 2),
        }
    except Exception as e:
        return {"name": name + "(ERR)", "err": str(e)}


def find_first(prefix):
    matches = sorted(
        glob.glob(f"runner/statusui/debug_six_named/panel_{prefix}_*.png")
        + glob.glob(f"runner/statusui/debug/panel_{prefix}_*.png")
    )
    return matches[0] if matches else None


files = []
files.append(("runner/statusui/StatusPanel.png", "TEMPLATE"))

# True positives (golden, drift1, aa, gg)
for prefix in ["status_bar_drift1", "aa", "gg"]:
    p = find_first(prefix)
    if p:
        files.append((p, f"TP_{prefix}"))

# False positives (skill panel / hotbar / window UI)
for prefix in ["ii", "jj", "pp", "tt", "assasincrossskill", "zoomed1"]:
    p = find_first(prefix)
    if p:
        files.append((p, f"FP_{prefix}"))

labels = ["name", "blue%", "red%", "white%", "dark%", "TL_con", "TR_con", "edge%", "topred%"]
print(" | ".join(f"{l:>10}" for l in labels))
print("-" * (12 * len(labels)))

# True-positive rows first, false positives second
files.sort(key=lambda x: (0 if x[1].startswith("TP") or x[1] == "TEMPLATE" else 1, x[1]))

for path, name in files:
    s = stats(path, name)
    if "err" in s:
        print(f"{s['name']:>10} | {s['err']}")
        continue
    print(" | ".join(f"{str(s[l]):>10}" for l in labels))
