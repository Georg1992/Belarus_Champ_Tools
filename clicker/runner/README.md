# `clicker/runner` — public facade

This package is the **only runner-level import** the GUI (or any other
consumer) is allowed to use. Everything else lives in subpackages that
can refactor freely — only what is re-exported here is stable.

## Layering

```
clicker/gui                  ← consumer
       │ imports
       ▼
clicker/runner (this pkg)    ← PUBLIC FACADE
       │ re-exports
       ▼
clicker/runner/autopot
clicker/runner/statusui
clicker/runner/internal/lifecycle
clicker/runner/internal/timing
clicker/runner/internal/session
clicker/runner/platform/windows
```

The facade is the **stability contract**. The subpackages are allowed
to reorganize freely (rename, split, merge) as long as the public
re-exports in this package keep their names and signatures.

## Re-exports

Subpackage re-exports (the "From subpackages" table below) live
inside a single `type ( ... )` block in `runner.go`. Add new aliases
inside that block, not as bare top-level statements (which would be a
syntax error). The top-level facade files (`clicker.go`, `keychain.go`,
`timer_key.go`, `keys.go`, `viiper_session.go`, `timing.go`) define
their symbols directly — no `type ( ... )` block there.

### From `clicker` (top-level facade files)

| Symbol | Kind | Source file |
|---|---|---|
| `ClickerSlotCount` | const | `clicker.go` |
| `DefaultDelayMs` | const | `clicker.go` |
| `ClickerSlot` | type | `clicker.go` |
| `Config` | type | `clicker.go` |
| `Runner` | type | `clicker.go` |
| `New(cfg Config) *Runner` | func | `clicker.go` |
| `KeyChainSlotCount` | const | `keychain.go` |
| `KeyChainConfig` | type | `keychain.go` |
| `KeyChainRunner` | type | `keychain.go` |
| `NewKeyChain(cfg KeyChainConfig) *KeyChainRunner` | func | `keychain.go` |
| `TimerKeySlotCount` | const | `timer_key.go` |
| `DefaultTimerKeyIntervalSec` | const | `timer_key.go` |
| `DefaultTimerKeyIntervalMs` | const | `timer_key.go` |
| `TimerSlot` | type | `timer_key.go` |
| `TimerKeyConfig` | type | `timer_key.go` |
| `TimerKeyRunner` | type | `timer_key.go` |
| `NewTimerKey(cfg TimerKeyConfig) *TimerKeyRunner` | func | `timer_key.go` |
| `KeysText(vks []int32) string` | func | `keys.go` |
| `KeyName(vk int32) string` | func | `keys.go` |
| `VKToHID(vk int32) (uint8, bool)` | func | `keys.go` |
| `WaitForKeyPress(timeout time.Duration) (int32, bool)` | func | `keys.go` |
| `ViiperSession` | type | `viiper_session.go` |
| `OpenViiperSession(ctx, apiAddr)` | func | `viiper_session.go` |
| `KeyBindTimeout` | const | `timing.go` |
| `DefaultAPIAddr` | const | `timing.go` |

### From subpackages (re-exported in `runner.go`)

| Symbol | Source |
|---|---|
| `InputSession` (alias) | `internal/session` |
| `Lifecycle[C]` (alias) | `internal/lifecycle` |
| `AutoPotConfig`, `AutoPotRunner`, `NewAutoPot` | `autopot` |

## Rectangle types

Two rectangle types exist, each scoped to its own layer:

- **`autopot.Rect`** — bar-detection ROI used by `PlayerBarSearchROI()`
  and the colour-run scanner. Lives in the detection layer.
- **`runner.ScreenROI`** (`platform/windows`) — screen-capture ROI passed
  to `CaptureScreenRegion`. A GDI plumbing detail, not re-exported
  through this facade.
