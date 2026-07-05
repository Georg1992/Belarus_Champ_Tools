//go:build windows

//go:generate go run github.com/akavel/rsrc@v0.10.2 -manifest app.manifest -o rsrc.syso

// Package main is the walk-based Windows GUI for the clicker. It is
// the topmost layer of a three-layer architecture; see README.md in
// this directory for the full layering rules and the import boundary.
//
// Quick rule: this package must only import `belarus-champ-tools/runner`
// (the public facade). Never import `runner/autopot`, `runner/autopot/statusui`,
// `runner/internal/...`, or `runner/platform/...` directly — add the
// missing surface to `runner` first, then consume it here.
package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime/debug"
	"sync"
	"sync/atomic"
	"time"

	"belarus-champ-tools/runner"

	"github.com/lxn/walk"
)

type guiApp struct {
	mainWindow *walk.MainWindow
	logList    *walk.ListBox
	logItems   []string

	// Clicker tab
	clickerSlots           [runner.ClickerSlotCount]clickerSlotWidgets
	clickerTriggerVKs      [runner.ClickerSlotCount][]int32
	clickerBindingSlot     int
	clickerLastLoggedDelay [runner.ClickerSlotCount]int

	// Control panel
	startBtn      *walk.PushButton  // Tools Start
	stopBtn       *walk.PushButton  // Tools Stop
	viiperStartBtn *walk.PushButton // VIIPER Start
	toolsBadge    *toolsBadge
	viiperBadge   *viiperBadge

	// Timer keys (clicker tab)
	timerSlots        [runner.TimerKeySlotCount]timerSlotWidgets
	timerKeyVKs       [runner.TimerKeySlotCount]int32
	timerVisibleCount int
	timerAddBtn       *walk.PushButton
	timerBindingSlot  int

	// AutoPot tab
	hpEnabledCB     *walk.CheckBox
	spEnabledCB     *walk.CheckBox
	hpThresholdEdit *walk.LineEdit
	spThresholdEdit *walk.LineEdit
	hpKeyLabel      *walk.Label
	spKeyLabel      *walk.Label
	hpBindBtn       *walk.PushButton
	hpClearBtn      *walk.PushButton
	spBindBtn       *walk.PushButton
	spClearBtn      *walk.PushButton
	// AutoPot mode & memory reading
	autopotVisualRB   *walk.RadioButton // Visual mode (pixel/OCR)
	autopotAddressRB  *walk.RadioButton // Address-reading mode
	windowCB          *walk.ComboBox     // game window selector
	windowRefreshBtn  *walk.PushButton   // refresh window list
	profileCB         *walk.ComboBox     // server memory profile
	processPID        uint32             // selected game process PID; 0 = none
	windowList        []windowInfo       // cached window list for PID lookup

	// KeyChain tab
	keyChainSlots       [runner.KeyChainSlotCount]keyChainSlotWidgets
	keyChainKeyVKs      [runner.KeyChainSlotCount]int32
	keyChainClearBtn    *walk.PushButton
	keyChainBindingSlot int

	mu            sync.Mutex
	shutdownOnce  sync.Once
	bindingActive bool
	logFile      *os.File
	// starting is 1 while onStart's background goroutine is wiring
	// up runners. It is set on the GUI thread inside onStart and cleared
	// by either the goroutine on completion (success or fail) or by
	// onStop. While it is 1, isStarted reports true so sync*Settings
	// don't trigger secondary runner startups that would race with the
	// main one. The startup goroutine also re-checks this flag after
	// every slot publish so a Stop click during startup cancels any
	// not-yet-published work.
	starting       atomic.Int32
	startupCancel  context.CancelFunc // cancels in-flight startInBackground on Stop
	runner         *runner.Runner
	autopotRunner  *runner.AutoPotRunner
	timerKeyRunner *runner.TimerKeyRunner
	keyChainRunner *runner.KeyChainRunner
	inputSession   *runner.ViiperSession
	hpKeyVK        int32
	spKeyVK        int32
	hpThreshold    int
	spThreshold    int
	overlay              *statusOverlay
	viiperMonitor        *viiperMonitor
	isRefreshingWindows    bool // guard: suppress key-clearing during window list refresh
	prevAutoPotAddressMode bool // tracks previous AddressMode to detect changes while running
}

func main() {
	app := &guiApp{timerBindingSlot: -1, keyChainBindingSlot: -1, clickerBindingSlot: -1, bindingActive: false}
	defer app.shutdown()

	// Open a persistent log file in a logs/ directory next to the
	// executable so diagnostics survive GUI close. Best-effort — if
	// the file can't be created, logging still works in-memory.
	if exe, err := os.Executable(); err == nil {
		logDir := filepath.Join(filepath.Dir(exe), "logs")
		_ = os.MkdirAll(logDir, 0o755)
		logPath := filepath.Join(logDir, "app.log")
		if f, err := os.OpenFile(logPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644); err == nil {
			app.logFile = f
			// Stamp the first entry so the user knows where to look.
			_, _ = f.WriteString(fmt.Sprintf("[%s] Log file: %s\n", time.Now().Format("15:04:05"), logPath))
		}
	}

	if err := app.createWindow(); err != nil {
		walk.MsgBox(nil, "BELARUS CHAMP TOOLS", err.Error(), walk.MsgBoxIconError)
	}
}

func (a *guiApp) shutdown() {
	a.shutdownOnce.Do(func() {
		a.mu.Lock()
		r := a.runner
		ap := a.autopotRunner
		tk := a.timerKeyRunner
		kc := a.keyChainRunner
		session := a.inputSession
		a.runner = nil
		a.autopotRunner = nil
		a.timerKeyRunner = nil
		a.keyChainRunner = nil
		a.inputSession = nil
		if a.startupCancel != nil {
			a.startupCancel()
			a.startupCancel = nil
		}
		a.mu.Unlock()

		if a.viiperMonitor != nil {
			a.viiperMonitor.stop()
			a.viiperMonitor = nil
		}

		if a.logFile != nil {
			_ = a.logFile.Close()
			a.logFile = nil
		}

		if r != nil {
			r.Stop()
			r.Wait()
		}
		if ap != nil {
			ap.Stop()
			ap.Wait()
		}
		if tk != nil {
			tk.Stop()
			tk.Wait()
		}
		if kc != nil {
			kc.Stop()
			kc.Wait()
		}
		if session != nil {
			session.Close()
			stopViiperServerIfStarted()
		}

		if a.overlay != nil {
			a.overlay.Destroy()
			a.overlay = nil
		}
	})
}

// ---------------------------------------------------------------------------
// createWindow and initialisation phases
// ---------------------------------------------------------------------------

func (a *guiApp) createWindow() error {
	mw := a.initMainWindow()
	if err := a.setupMainWindow(mw); err != nil {
		return err
	}
	a.startBackgroundMonitors()
	if err := a.initTabs(mw); err != nil {
		return err
	}
	if err := a.initLogArea(mw); err != nil {
		return err
	}
	a.wireClosingHandler(mw)
	a.setInitialState()
	a.onStartViiper()

	mw.Show()
	mw.Run()
	return nil
}

// initMainWindow creates the main window and the HP/SP overlay.
func (a *guiApp) initMainWindow() *walk.MainWindow {
	mw, err := walk.NewMainWindow()
	if err != nil {
		panic(err) // must not fail
	}
	a.mainWindow = mw

	if ovl, ovlErr := newStatusOverlay(); ovlErr == nil {
		a.overlay = ovl
	}
	return mw
}

// setupMainWindow sets title, size, layout, icon, header, and control panel.
func (a *guiApp) setupMainWindow(mw *walk.MainWindow) error {
	if err := a.setupLogLimit(); err != nil {
		return err
	}
	if err := mw.SetTitle("BELARUS CHAMP TOOLS"); err != nil {
		return err
	}
	if err := mw.SetMinMaxSize(walk.Size{Width: 780, Height: 600}, walk.Size{}); err != nil {
		return err
	}
	if err := mw.SetSize(walk.Size{Width: 780, Height: 600}); err != nil {
		return err
	}

	root := walk.NewVBoxLayout()
	root.SetMargins(walk.Margins{HNear: 10, VNear: 10, HFar: 10, VFar: 10})
	root.SetSpacing(10)
	if err := mw.SetLayout(root); err != nil {
		return err
	}

	icon, err := walk.NewIconFromImageForDPI(belarusFlagImage(), 96)
	if err != nil {
		return err
	}
	if err := mw.SetIcon(icon); err != nil {
		return err
	}
	if err := addBelarusHeader(mw); err != nil {
		return err
	}
	return a.buildControlPanel(mw)
}

// startBackgroundMonitors starts the VIIPER connectivity monitor and the
// End-key toggle watcher. Both run for the lifetime of the app.
func (a *guiApp) startBackgroundMonitors() {
	a.viiperMonitor = startViiperMonitor(context.Background(), func(active bool) {
		a.mainWindow.Synchronize(func() {
			if active {
				a.viiperBadge.SetStatus(viiperActive)
				return
			}
			a.viiperBadge.SetStatus(viiperInactive)
			if a.isStarted() {
				a.appendLog("VIIPER server disconnected — stopping tools")
				a.onStop()
			}
			a.mu.Lock()
			if a.inputSession != nil {
				a.inputSession.Close()
				a.inputSession = nil
			}
			a.mu.Unlock()
			a.startBtn.SetEnabled(false)
			a.stopBtn.SetEnabled(false)
			a.setConfigEnabled(false)
			a.viiperStartBtn.SetEnabled(true)
		})
	})

	runner.StartEndKeyWatcher(context.Background(), func() {
		a.mainWindow.Synchronize(func() {
			if a.isStarted() {
				a.appendLog("End pressed — stopping tools")
				a.onStop()
			} else {
				a.onStart()
			}
		})
	})
}

// initTabs creates the Clicker, AutoPot, and KeyChain tab pages and wires
// tab-change and deactivating handlers for threshold blur.
func (a *guiApp) initTabs(mw *walk.MainWindow) error {
	tabs, err := walk.NewTabWidget(mw)
	if err != nil {
		return err
	}

	tabDefs := []struct {
		title string
		build func(*walk.TabPage) error
	}{
		{"Clicker", a.buildClickerTab},
		{"AutoPot", a.buildAutoPotTab},
		{"KeyChain", a.buildKeyChainTab},
	}

	for _, td := range tabDefs {
		page, err := walk.NewTabPage()
		if err != nil {
			return err
		}
		if err := page.SetTitle(td.title); err != nil {
			return err
		}
		if err := td.build(page); err != nil {
			return err
		}
		if err := tabs.Pages().Add(page); err != nil {
			return err
		}
	}

	tabs.CurrentIndexChanged().Attach(a.finishThresholdInput)
	mw.Deactivating().Attach(a.finishThresholdInput)
	return nil
}

// initLogArea creates the log label and list box at the bottom of the window.
func (a *guiApp) initLogArea(mw *walk.MainWindow) error {
	logLabel, err := walk.NewLabel(mw)
	if err != nil {
		return err
	}
	if err := logLabel.SetText("Logs"); err != nil {
		return err
	}

	a.logList, err = walk.NewListBox(mw)
	if err != nil {
		return err
	}
	if err := a.logList.SetMinMaxSize(walk.Size{Width: 0, Height: 140}, walk.Size{}); err != nil {
		return err
	}
	a.logItems = make([]string, 0, maxLogItems)
	if err := a.logList.SetModel(a.logItems); err != nil {
		return err
	}
	a.wireThresholdBlurOnClick(mw)
	return nil
}

// wireClosingHandler attaches the shutdown handler to the window close event.
func (a *guiApp) wireClosingHandler(mw *walk.MainWindow) {
	mw.Closing().Attach(func(canceled *bool, reason walk.CloseReason) {
		a.shutdown()
	})
}

// setInitialState disables everything except Start VIIPER.
func (a *guiApp) setInitialState() {
	a.viiperBadge.SetStatus(viiperInactive)
	a.toolsBadge.SetStatus(toolsStatusStopped)
	a.viiperStartBtn.SetEnabled(true)
	a.startBtn.SetEnabled(false)
	a.stopBtn.SetEnabled(false)
	a.setConfigEnabled(false)
}

// isViiperReady reports whether VIIPER is running with an active session.
// This is the minimum requirement for key binding and tools operation.
func (a *guiApp) isViiperReady() bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.inputSession != nil
}

// maxLogItems is the maximum number of log entries kept in memory to
// prevent unbounded memory growth during long sessions.
const maxLogItems = 500

// setupLogLimit attaches a timer that trims the log items slice on the
// GUI thread every 30 seconds, ensuring old entries are dropped when the
// log exceeds maxLogItems. This prevents the in-memory log from growing
// unboundedly over hours of use.
func (a *guiApp) setupLogLimit() error {
	t := time.NewTicker(30 * time.Second)
	go func() {
		defer func() {
			if r := recover(); r != nil {
				_, _ = fmt.Fprintf(os.Stderr, "PANIC in log trimmer: %v\n%s\n", r, debug.Stack())
			}
		}()
		defer t.Stop()
		for range t.C {
			if a.logList == nil {
				continue
			}
			a.mainWindow.Synchronize(func() {
				if len(a.logItems) > maxLogItems {
					excess := len(a.logItems) - maxLogItems
					a.logItems = a.logItems[excess:]
					_ = a.logList.SetModel(a.logItems)
					_ = a.logList.SetCurrentIndex(len(a.logItems) - 1)
				}
			})
		}
	}()
	return nil
}

func (a *guiApp) appendLog(line string) {
	stamped := fmt.Sprintf("[%s] %s", time.Now().Format("15:04:05"), line)

	// Write to persistent log file (best-effort — file may be missing).
	if a.logFile != nil {
		_, _ = a.logFile.WriteString(stamped + "\n")
	}

	if a.logList == nil {
		return
	}
	a.logItems = append(a.logItems, stamped)
	// UI update errors are not critical; log display may fail but log entry is recorded
	_ = a.logList.SetModel(a.logItems)
	if len(a.logItems) > 0 {
		_ = a.logList.SetCurrentIndex(len(a.logItems) - 1)
	}
}

func (a *guiApp) isStarted() bool {
	// Fast path: startup in flight — no mutex needed (atomic load).
	if a.starting.Load() != 0 {
		return true
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.runner != nil && a.runner.Running() {
		return true
	}
	if a.autopotRunner != nil && a.autopotRunner.Running() {
		return true
	}
	return false
}

// setConfigEnabled enables or disables all tab configuration (clicker slots,
// autopot, timer keys, keychain). Called when VIIPER state changes — config
// is enabled when VIIPER is running, disabled when VIIPER is down.
func (a *guiApp) setConfigEnabled(enabled bool) {
	a.setClickerConfigEnabled(enabled)
	a.setAutoPotConfigEnabled(enabled)
	a.setTimerKeyConfigEnabled(enabled)
	a.setKeyChainConfigEnabled(enabled)
}

// setToolsStarted updates the TOOLS badge and Start/Stop button states.
// Does NOT touch config enable/disable — that's managed by VIIPER state.
// MUST be called on the GUI thread.
func (a *guiApp) setToolsStarted(started bool) {
	a.startBtn.SetEnabled(!started)
	a.stopBtn.SetEnabled(started)
	if started {
		a.toolsBadge.SetStatus(toolsStatusRunning)
	} else {
		a.toolsBadge.SetStatus(toolsStatusStopped)
	}
}

// ---------------------------------------------------------------------------
// VIIPER lifecycle
// ---------------------------------------------------------------------------

// onStartViiper starts the VIIPER server and opens an input session. Called
// from the VIIPER Start button. Runs the blocking startup on a background
// goroutine so the GUI stays responsive.
func (a *guiApp) onStartViiper() {
	a.viiperStartBtn.SetEnabled(false)
	a.appendLog("Starting VIIPER server...")

	go func() {
		defer func() {
			if r := recover(); r != nil {
				_, _ = fmt.Fprintf(os.Stderr, "PANIC in onStartViiper: %v\n%s\n", r, debug.Stack())
			}
		}()
		logFn := func(s string) {
			a.mainWindow.Synchronize(func() { a.appendLog(s) })
		}

		_, err := ensureViiperServer(context.Background(), logFn)
		if err != nil {
			a.mainWindow.Synchronize(func() {
				a.appendLog(fmt.Sprintf("VIIPER start failed: %v", err))
				a.viiperStartBtn.SetEnabled(true) // retry
			})
			return
		}

		logFn("Opening VIIPER session...")
		session, err := runner.OpenViiperSession(context.Background(), runner.DefaultAPIAddr, logFn)
		if err != nil {
			stopViiperServerIfStarted()
			a.mainWindow.Synchronize(func() {
				a.appendLog(fmt.Sprintf("VIIPER session failed: %v", err))
				a.viiperStartBtn.SetEnabled(true) // retry
			})
			return
		}

		a.mu.Lock()
		// Close any stale session before replacing it.
		if a.inputSession != nil {
			a.inputSession.Close()
		}
		a.inputSession = session
		a.mu.Unlock()

		a.mainWindow.Synchronize(func() {
			a.viiperBadge.SetStatus(viiperActive)
			a.appendLog("VIIPER server ready")
			// VIIPER is running — enable config and Tools Start button.
			a.setConfigEnabled(true)
			a.startBtn.SetEnabled(true)
			a.stopBtn.SetEnabled(false)
			a.viiperStartBtn.SetEnabled(false) // already running
		})
	}()
}

// ---------------------------------------------------------------------------
// Tools lifecycle
// ---------------------------------------------------------------------------

// onStart is the Tools Start button click handler. It assumes VIIPER is
// already running (inputSession is non-nil) and starts all runners.
// The blocking portion runs on a background goroutine so the GUI stays
// responsive during session wiring.
func (a *guiApp) onStart() {
	a.mu.Lock()
	if a.inputSession == nil {
		a.mu.Unlock()
		a.appendLog("Cannot start tools — VIIPER is not running. Start VIIPER first.")
		return
	}
	if (a.runner != nil && a.runner.Running()) || (a.autopotRunner != nil && a.autopotRunner.Running()) {
		a.mu.Unlock()
		return
	}
	if ready, msg := inputDriverReady(); !ready {
		a.mu.Unlock()
		a.appendLog("Input driver not ready — see Setup required dialog.")
		walk.MsgBox(a.mainWindow, "Setup required", msg, walk.MsgBoxIconWarning)
		return
	}
	// Cancel any previous startup goroutine that is still running.
	if a.startupCancel != nil {
		a.startupCancel()
		a.startupCancel = nil
	}
	ctx, cancel := context.WithCancel(context.Background())
	a.startupCancel = cancel
	a.starting.Store(1)
	a.mu.Unlock()

	// Immediate UI feedback.
	a.setToolsStarted(true)
	a.appendLog("Starting tools...")

	go a.startInBackground(ctx)
}

// startInBackground runs the long-running tools startup work off the GUI
// thread. VIIPER is already running — this only wires up the runners.
func (a *guiApp) startInBackground(ctx context.Context) {
	logFn := func(s string) {
		a.mainWindow.Synchronize(func() { a.appendLog(s) })
	}
	isStillStarting := func() bool {
		if ctx.Err() != nil {
			return false
		}
		return a.starting.Load() != 0
	}
	finishFailure := func() {
		if ctx.Err() != nil {
			return // superseded by a newer Start
		}
		a.starting.Swap(0)
		a.mainWindow.Synchronize(func() { a.setToolsStarted(false) })
	}

	// Use the existing VIIPER session (already set up by onStartViiper).
	a.mu.Lock()
	session := a.inputSession
	a.mu.Unlock()

	if session == nil {
		logFn("Cannot start — VIIPER session is nil")
		finishFailure()
		return
	}

	logFn("Reusing VIIPER session...")
	session.Reset()

	if !isStillStarting() {
		session.Reset()
		return
	}

	cfg := a.clickerConfig()
	cfg.Session = session
	cfg.Log = logFn

	r := runner.New(cfg)
	if err := r.Start(); err != nil {
		session.Close()
		a.mu.Lock()
		a.inputSession = nil
		a.mu.Unlock()
		logFn(fmt.Sprintf("Start failed: %v", err))
		stopViiperServerIfStarted()
		finishFailure()
		return
	}

	a.mu.Lock()
	a.runner = r
	a.mu.Unlock()
	if !isStillStarting() {
		r.Stop()
		r.Wait()
		session.Close()
		a.mu.Lock()
		a.runner = nil
		a.inputSession = nil
		a.mu.Unlock()
		stopViiperServerIfStarted()
		return
	}

	a.startRemainingRunners(session, logFn)

	// atomically read+clear the starting flag so onStop can't race
	// between the two operations.
	wasStarting := a.starting.Swap(0)
	if wasStarting == 0 {
		return // onStop already cleared starting before we could finish
	}

	a.mainWindow.Synchronize(func() { a.setToolsStarted(true) })
	logFn("Tools started")
}

// startRemainingRunners starts AutoPot, TimerKey, and KeyChain runners.
func (a *guiApp) startRemainingRunners(session runner.InputSession, logFn func(string)) {
	autopotCfg := a.autopotWanted()
	autopotCfg.Core.Session = session
	autopotCfg.Core.Log = logFn
	timerCfg := a.timerKeyWanted()
	timerCfg.Session = session
	timerCfg.Log = logFn

	keyChainCfg := a.keyChainConfig()
	keyChainCfg.Session = session
	keyChainCfg.Log = logFn

	a.prevAutoPotAddressMode = autopotCfg.IsAddressMode()
	a.startAutoPotRunner(autopotCfg, logFn)

	// If no autopot keys are bound, show "AutoPot off" instead of a stale mode.
	if !autopotCfg.Core.HPEnabled && !autopotCfg.Core.SPEnabled {
		a.mainWindow.Synchronize(func() {
			if a.overlay != nil {
				a.overlay.SetMode("AutoPot off")
			}
		})
	}

	a.startTimerKeyRunner(timerCfg, logFn)
	a.startKeyChainRunner(keyChainCfg, logFn)
}

// onStop stops all tools but keeps the VIIPER session alive so the next
// Start reuses it. The blocking Stop+Wait runs on a background goroutine.
// VIIPER server is NOT stopped — it stays running for reuse.
func (a *guiApp) onStop() {
	a.mu.Lock()
	r := a.runner
	ap := a.autopotRunner
	tk := a.timerKeyRunner
	kc := a.keyChainRunner
	session := a.inputSession
	a.runner = nil
	a.autopotRunner = nil
	a.timerKeyRunner = nil
	a.keyChainRunner = nil
	// Keep a.inputSession alive so the next Start reuses it.
	// Full cleanup (Close) happens in shutdown().
	// Cancel any in-flight startup goroutine.
	a.starting.Store(0)
	if a.startupCancel != nil {
		a.startupCancel()
		a.startupCancel = nil
	}
	a.mu.Unlock()

	a.setToolsStarted(false)
	a.appendLog("Stopping tools...")

	go func() {
		defer func() {
			if r := recover(); r != nil {
				_, _ = fmt.Fprintf(os.Stderr, "PANIC in onStop: %v\n%s\n", r, debug.Stack())
			}
		}()
		if r != nil {
			r.Stop()
			r.Wait()
		}
		if ap != nil {
			ap.Stop()
			ap.Wait()
		}
		if tk != nil {
			tk.Stop()
			tk.Wait()
		}
		if kc != nil {
			kc.Stop()
			kc.Wait()
		}
		if session != nil {
			session.Reset()
			// Keep the session alive; the next Start reuses it.
			// Full cleanup (Close) happens in shutdown().
		}
		a.mainWindow.Synchronize(func() {
			a.appendLog("Tools stopped — Start to relaunch")
			if a.overlay != nil {
				a.overlay.ShowStopped()
			}
		})
	}()
}

func (a *guiApp) autopotWanted() runner.AutoPotConfig {
	cfg := a.autopotConfig()
	cfg.Core.HPEnabled = cfg.Core.HPEnabled && cfg.Core.HPKeyVK != 0
	cfg.Core.SPEnabled = cfg.Core.SPEnabled && cfg.Core.SPKeyVK != 0
	return cfg
}

func (a *guiApp) startAutoPotRunner(cfg runner.AutoPotConfig, log func(string)) {
	take, store := makeLifecycleSlot[*runner.AutoPotRunner](&a.mu, &a.autopotRunner)
	startLifecycle(
		take, store,
		"AutoPot",
		log,
		func() runner.InputSession {
			a.mu.Lock()
			defer a.mu.Unlock()
			return a.inputSession
		},
		func() bool { return cfg.Core.HPEnabled || cfg.Core.SPEnabled },
		func(sess runner.InputSession) *runner.AutoPotRunner {
			cfg.Core.Session = sess
			cfg.Core.Log = log
			return runner.NewAutoPot(cfg)
		},
	)
}

// guiLog wraps a function call in mainWindow.Synchronize so it always
// marshals to the GUI thread. Use for callbacks that are invoked from
// background goroutines but call Walk UI operations (e.g. appendLog).
func (a *guiApp) guiLog(fn func(string)) func(string) {
	return func(s string) {
		a.mainWindow.Synchronize(func() { fn(s) })
	}
}

// unsetKeyBinding searches every key storage location in the app for vk.
// If found, it clears the old binding (UI label + state), syncs the
// affected runner, and logs the change. Call this from any onPress
// handler BEFORE assigning the key to the new slot so a key can only
// ever be bound in one place at a time.
func (a *guiApp) unsetKeyBinding(vk int32) {
	// Check clicker slots (each slot can have multiple VKs).
	for i := 0; i < runner.ClickerSlotCount; i++ {
		for j, existing := range a.clickerTriggerVKs[i] {
			if existing == vk {
				a.clickerTriggerVKs[i] = append(a.clickerTriggerVKs[i][:j], a.clickerTriggerVKs[i][j+1:]...)
				a.updateClickerKeyLabel(i)
				a.appendLog(fmt.Sprintf("Key %s removed from %s (reassigned)", runner.KeyName(vk), clickerSlotTitles[i]))
				a.syncRunnerSettings()
				return
			}
		}
	}
	// Check timer keys.
	for i := 0; i < a.timerVisibleCount; i++ {
		if a.timerKeyVKs[i] == vk {
			a.timerKeyVKs[i] = 0
			a.timerSlots[i].keyLabel.SetText("none")
			a.appendLog(fmt.Sprintf("Key %s removed from Timer %d (reassigned)", runner.KeyName(vk), i+1))
			a.syncTimerKeySettings()
			return
		}
	}
	// Check HP potion.
	if a.hpKeyVK == vk {
		a.hpKeyVK = 0
		a.hpKeyLabel.SetText("none")
		a.appendLog(fmt.Sprintf("Key %s removed from HP potion (reassigned)", runner.KeyName(vk)))
		a.syncAutoPotSettings()
		return
	}
	// Check SP potion.
	if a.spKeyVK == vk {
		a.spKeyVK = 0
		a.spKeyLabel.SetText("none")
		a.appendLog(fmt.Sprintf("Key %s removed from SP potion (reassigned)", runner.KeyName(vk)))
		a.syncAutoPotSettings()
		return
	}
	// Check keychain slots.
	for i := 0; i < runner.KeyChainSlotCount; i++ {
		if a.keyChainKeyVKs[i] == vk {
			a.keyChainKeyVKs[i] = 0
			a.setKeyChainKeyText(i, 0)
			a.appendLog(fmt.Sprintf("Key %s removed from Chain slot %d (reassigned)", runner.KeyName(vk), i+1))
			a.syncKeyChainSettings()
			return
		}
	}
}

func (a *guiApp) syncRunnerSettings() {
	cfg := a.clickerConfig()
	a.mu.Lock()
	r := a.runner
	a.mu.Unlock()

	if r != nil && r.Running() {
		r.UpdateSettings(cfg.Slots)
	}
}
