# BELARUS CHAMP TOOLS

Windows clicker with a Walk GUI. Hold a trigger key to repeat virtual key presses and left mouse clicks through embedded [VIIPER](https://github.com/Alia5/VIIPER) virtual HID devices.

## Project layout

```
Belarus_Champ_Tools/
  app.exe                       ← dev build output
  app/
    build.ps1                    ← build app.exe
    package.ps1                  ← build user ZIP
    gui/                         ← Walk UI + embedded VIIPER server
    runner/                      ← click loop, key mappings, pause
  packaging/
    README.txt / README.ru.txt
    Install.cmd / Uninstall.cmd
  release/                           ← build output only (folder + zip)
    BelarusChampTools-Windows-x64/
    BelarusChampTools-Windows-x64.zip
  VIIPER/                        ← git submodule
```

Open **`Belarus_Champ_Tools`** in your editor — not the `VIIPER/` folder alone.

## Prerequisites

- Windows 64-bit
- Go 1.26+ (for building)
- [usbip-win2](https://github.com/vadimgrn/usbip-win2) kernel driver (one-time install + reboot)

The packaged `Install.cmd` installs the driver automatically.

## Build

```powershell
git submodule update --init --recursive
cd app
.\build.ps1
```

Output: `..\app.exe`

## Release package

```powershell
cd app
.\package.ps1
```

Output: `release/BelarusChampTools-Windows-x64/` and `release/BelarusChampTools-Windows-x64.zip`

Users extract the ZIP and run `Install.cmd`. See `packaging/README.txt`.

## Usage

1. Click **Start** before launching the game
2. Bind trigger keys and set delay
3. Hold a trigger key to click
4. Press **End** to pause or resume (server stays running)
5. Click **Stop** or close the app to turn off

### AutoPot tab

1. Keep the game visible with your character near **screen center** (green HP / blue SP bars under the sprite)
2. Set trigger **%** and assign **hotkeys** for HP and SP potions
3. Click **Start**

Bars under the character are detected by color in a small center region. When HP or SP drops below the threshold, the assigned key is pressed until the bar recovers.

Set `BAR_SEARCH_DEBUG=1` to save a `bar_search_debug.png` crop for calibration.

Status indicator: red **OFF**, green **ON**, yellow **PAUSE**.

### Click loop

While the trigger key is held:

1. Virtual key down
2. Delay (ms) — ends early if trigger released, but cycle still finishes
3. Virtual mouse down → key up → mouse up
4. Repeat until trigger released; current cycle always completes

Default delay: **50 ms**. If a game misses clicks, try **50–100 ms**.

## Troubleshooting

| Symptom | Fix |
|---------|-----|
| Setup required on Start | Run `Install.cmd`, reboot if prompted |
| Clicks not registered | Start clicker before the game; increase delay |
| Loop never triggers | Check physical trigger key works |

## Source map

| Path | Purpose |
|------|---------|
| `app/gui/main.go` | Main window, Start/Stop, status badge |
| `app/gui/status_badge.go` | ON / OFF / PAUSE indicator |
| `app/gui/server.go` | Embedded VIIPER lifecycle |
| `app/runner/runner.go` | Click loop, End-key pause |
| `app/runner/player_bars.go` | HP/SP bar detection under character (center crop) |
| `app/runner/autopot.go` | AutoPot healing loop |
| `app/runner/screen_windows.go` | Center screen region capture |
| `packaging/` | Install/Uninstall scripts and user READMEs |
| `release/` | Generated folder + ZIP (`package.ps1`) |
| `VIIPER/` | Upstream VIIPER (`replace` in `app/go.mod`) |
