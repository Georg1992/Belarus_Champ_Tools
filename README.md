# BELARUS CHAMP CLICKER

Windows clicker with a Walk GUI. Hold a trigger key to repeat virtual key presses and left mouse clicks through embedded [VIIPER](https://github.com/Alia5/VIIPER) virtual HID devices.

## Layout

```
Experimental_Clicker/
  clicker.exe                          ← dev build (double-click to run)
  clicker/                             ← Go source
    build.ps1                          ← build dev app
    build-author.ps1                   ← build licgen-gui.exe (author only)
    package.ps1                        ← build user ZIP
    cmd/ensurekeys/                    ← first-time license key bootstrap (build only)
    cmd/licgen-gui/                    ← GUI to issue activation codes (author only)
    gui/                               ← main app window
    runner/                            ← click loop + input
    license/                           ← activation (Ed25519, machine-bound)
  release/                             ← INSTALL*.txt, Install.cmd, LICENSE_AUTHOR*.txt
  VIIPER/                              ← git submodule
```

Open **`Experimental_Clicker`** in your editor — not the `VIIPER/` folder alone.

## Prerequisites

- Windows 64-bit
- Go 1.26+ (for building)
- [usbip-win2](https://github.com/vadimgrn/usbip-win2) kernel driver (one-time install + reboot)

The user `Install.cmd` in the release package installs the driver automatically.

## Build

```powershell
git submodule update --init --recursive
cd clicker
.\build.ps1
```

Output: `..\clicker.exe`

Author tool (issue activation codes, no console):

```powershell
.\build-author.ps1
```

Output: `licgen-gui.exe` — see `release/LICENSE_AUTHOR.txt`

User release ZIP:

```powershell
.\package.ps1
```

Output: `release/BelarusChampClicker-Windows-x64.zip` containing `Belarus Champ Clicker.exe`

## Run

1. Activate on first launch (paste code from seller)
2. Click **Start** before launching the game
3. Bind trigger keys, set delay, hold trigger to click

### Click loop

While the trigger key is held (`runner/runner.go`):

1. Virtual key down
2. Delay (ms) — ends early if trigger released, but cycle still finishes
3. Virtual mouse down → key up → mouse up
4. Repeat until trigger released; current cycle always completes

Default delay: **50 ms**. If a game misses clicks, try **50–100 ms** and start the clicker before the game.

## Troubleshooting

| Symptom | Fix |
|---------|-----|
| Setup required on Start | Run `Install.cmd` from the release package, reboot |
| Activation failed | Code is tied to one PC; request a new code for that Computer ID |
| Clicks not registered | Start clicker before the game; increase delay |
| Loop never triggers | Check physical trigger key works |

## Development

| Path | Purpose |
|------|---------|
| `clicker/gui/` | Walk GUI, activation, embedded VIIPER server |
| `clicker/runner/` | VIIPER client, click loop, key mappings |
| `clicker/license/` | Machine ID, signed activation codes |
| `clicker/cmd/licgen-gui/` | Author GUI for issuing codes |
| `VIIPER/` | Upstream VIIPER (`replace` in `clicker/go.mod`) |
