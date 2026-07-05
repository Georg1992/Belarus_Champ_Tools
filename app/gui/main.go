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

	// Tab controllers (each manages its own tab state + runner).
	clicker  *clickerTabController
	autopot  *autopotTabController
	timer    *timerKeyTabController
	keyChain *keyChainTabController

	// Control panel
	startBtn       *walk.PushButton  // Tools Start
	stopBtn        *walk.PushButton  // Tools Stop
	viiperStartBtn *walk.PushButton  // VIIPER Start
	toolsBadge     *toolsBadge
	viiperBadge    *viiperBadge

	mu            sync.Mutex
	shutdownOnce  sync.Once
	bindingActive bool
	logFile       *os.File
	// starting is 1 while onStart's background goroutine is wiring
	// up runners. It is set on the GUI thread inside onStart and cleared
	// by either the goroutine on completion (success or fail) or by
	// onStop. While it is 1, isStarted reports true so sync*Settings
	// don't trigger secondary runner startups that would race with the
	// main one. The startup goroutine also re-checks this flag after
	// every slot publish so a Stop click during startup cancels any
	// not-yet-published work.
	starting      atomic.Int32
	startupCancel context.CancelFunc // cancels in-flight startInBackground on Stop
	inputSession  *runner.ViiperSession
	overlay       *statusOverlay
	viiperMonitor *viiperMonitor
}

func main() {
	app := &guiApp{bindingActive: false}
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
		session := a.inputSession
		a.inputSession = nil
		clicker := a.clicker
		autopot := a.autopot
		timer := a.timer
		keyChain := a.keyChain
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

		// Stop runners via their controllers.
		if clicker != nil && clicker.runner != nil {
			clicker.runner.Stop()
			clicker.runner.Wait()
		}
		if autopot != nil && autopot.runner != nil {
			autopot.runner.Stop()
			autopot.runner.Wait()
		}
		if timer != nil && timer.runner != nil {
			timer.runner.Stop()
			timer.runner.Wait()
		}
		if keyChain != nil && keyChain.runner != nil {
			keyChain.runner.Stop()
			keyChain.runner.Wait()
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
	a.initControllers()
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

// initControllers creates the tab controllers and wires the shared tabContext.
func (a *guiApp) initControllers() {
	ctx := &tabContext{
		mu:     &a.mu,
		window: a.mainWindow,
		getSession: func() runner.InputSession {
			a.mu.Lock()
			defer a.mu.Unlock()
			return a.inputSession
		},
		appendLog:     a.appendLog,
		overlay:       a.overlay,
		isStarted:     a.isStarted,
		isViiperReady: a.isViiperReady,
		bindActive:    &a.bindingActive,
	}

	a.clicker = newClickerTabController(ctx)
	a.autopot = newAutopotTabController(ctx)
	a.timer = newTimerKeyTabController(ctx)
	a.keyChain = newKeyChainTabController(ctx)

	// Wire unsetBinding after all controllers exist so it queries each in order.
	ctx.unsetBinding = func(vk int32) {
		if a.clicker.unsetBinding(vk) {
			return
		}
		if a.timer.unsetBinding(vk) {
			return
		}
		if a.autopot.unsetBinding(vk) {
			return
		}
		if a.keyChain.unsetBinding(vk) {
			return
		}
	}
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

// initTabs creates the Clicker, AutoPot, and KeyChain tab pages via controllers.
func (a *guiApp) initTabs(mw *walk.MainWindow) error {
	tabs, err := walk.NewTabWidget(mw)
	if err != nil {
		return err
	}

	tabDefs := []struct {
		title string
		build func(*walk.TabPage) error
	}{
		{"Clicker", func(page *walk.TabPage) error {
			return a.clicker.build(page, a.timer)
		}},
		{"AutoPot", a.autopot.build},
		{"KeyChain", a.keyChain.build},
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

	tabs.CurrentIndexChanged().Attach(func() {
		a.autopot.finishThresholdInput()
	})
	mw.Deactivating().Attach(func() {
		a.autopot.finishThresholdInput()
	})
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
	a.autopot.wireThresholdBlurOnClick(mw, a.logList)
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
func (a *guiApp) isViiperReady() bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.inputSession != nil
}

// maxLogItems is the maximum number of log entries kept in memory.
const maxLogItems = 500

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
	if a.logFile != nil {
		_, _ = a.logFile.WriteString(stamped + "\n")
	}
	if a.logList == nil {
		return
	}
	a.logItems = append(a.logItems, stamped)
	_ = a.logList.SetModel(a.logItems)
	if len(a.logItems) > 0 {
		_ = a.logList.SetCurrentIndex(len(a.logItems) - 1)
	}
}

func (a *guiApp) isStarted() bool {
	if a.starting.Load() != 0 {
		return true
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.clicker != nil && a.clicker.runner != nil && a.clicker.runner.Running() {
		return true
	}
	if a.autopot != nil && a.autopot.runner != nil && a.autopot.runner.Running() {
		return true
	}
	return false
}

// setConfigEnabled enables or disables all tab configuration.
func (a *guiApp) setConfigEnabled(enabled bool) {
	if a.clicker != nil {
		a.clicker.setEnabled(enabled)
	}
	if a.autopot != nil {
		a.autopot.setEnabled(enabled)
	}
	if a.timer != nil {
		a.timer.setEnabled(enabled)
	}
	if a.keyChain != nil {
		a.keyChain.setEnabled(enabled)
	}
}

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
				a.viiperStartBtn.SetEnabled(true)
			})
			return
		}

		logFn("Opening VIIPER session...")
		session, err := runner.OpenViiperSession(context.Background(), runner.DefaultAPIAddr, logFn)
		if err != nil {
			stopViiperServerIfStarted()
			a.mainWindow.Synchronize(func() {
				a.appendLog(fmt.Sprintf("VIIPER session failed: %v", err))
				a.viiperStartBtn.SetEnabled(true)
			})
			return
		}

		a.mu.Lock()
		if a.inputSession != nil {
			a.inputSession.Close()
		}
		a.inputSession = session
		a.mu.Unlock()

		a.mainWindow.Synchronize(func() {
			a.viiperBadge.SetStatus(viiperActive)
			a.appendLog("VIIPER server ready")
			a.setConfigEnabled(true)
			a.startBtn.SetEnabled(true)
			a.stopBtn.SetEnabled(false)
			a.viiperStartBtn.SetEnabled(false)
		})
	}()
}

// ---------------------------------------------------------------------------
// Tools lifecycle
// ---------------------------------------------------------------------------

func (a *guiApp) onStart() {
	a.mu.Lock()
	if a.inputSession == nil {
		a.mu.Unlock()
		a.appendLog("Cannot start tools — VIIPER is not running. Start VIIPER first.")
		return
	}
	if a.isStarted() {
		a.mu.Unlock()
		return
	}
	if ready, msg := inputDriverReady(); !ready {
		a.mu.Unlock()
		a.appendLog("Input driver not ready — see Setup required dialog.")
		walk.MsgBox(a.mainWindow, "Setup required", msg, walk.MsgBoxIconWarning)
		return
	}
	if a.startupCancel != nil {
		a.startupCancel()
		a.startupCancel = nil
	}
	ctx, cancel := context.WithCancel(context.Background())
	a.startupCancel = cancel
	a.starting.Store(1)
	a.mu.Unlock()

	a.setToolsStarted(true)
	a.appendLog("Starting tools...")

	go a.startInBackground(ctx)
}

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
			return
		}
		a.starting.Swap(0)
		a.mainWindow.Synchronize(func() { a.setToolsStarted(false) })
	}

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
		return
	}

	cfg := a.clicker.config()
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
	a.clicker.runner = r
	a.mu.Unlock()
	if !isStillStarting() {
		r.Stop()
		r.Wait()
		session.Close()
		a.mu.Lock()
		a.clicker.runner = nil
		a.inputSession = nil
		a.mu.Unlock()
		stopViiperServerIfStarted()
		return
	}

	a.startRemainingRunners(session, logFn)

	wasStarting := a.starting.Swap(0)
	if wasStarting == 0 {
		return
	}

	a.mainWindow.Synchronize(func() { a.setToolsStarted(true) })
	logFn("Tools started")
}

func (a *guiApp) startRemainingRunners(session runner.InputSession, logFn func(string)) {
	autopotCfg := a.autopot.wanted()
	autopotCfg.Core.Session = session
	autopotCfg.Core.Log = logFn
	timerCfg := a.timer.wanted()
	timerCfg.Session = session
	timerCfg.Log = logFn
	keyChainCfg := a.keyChain.config()
	keyChainCfg.Session = session
	keyChainCfg.Log = logFn

	a.autopot.prevAddressMode = autopotCfg.IsAddressMode()
	a.autopot.startRunner(autopotCfg, logFn)

	if !autopotCfg.Core.HPEnabled && !autopotCfg.Core.SPEnabled {
		a.mainWindow.Synchronize(func() {
			if a.overlay != nil {
				a.overlay.SetMode("AutoPot off")
			}
		})
	}

	a.timer.startRunner(timerCfg, logFn)
	a.keyChain.startRunner(keyChainCfg, logFn)
}

func (a *guiApp) onStop() {
	a.mu.Lock()
	clicker := a.clicker
	autopot := a.autopot
	timer := a.timer
	keyChain := a.keyChain
	session := a.inputSession
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
		if clicker != nil && clicker.runner != nil {
			clicker.runner.Stop()
			clicker.runner.Wait()
		}
		if autopot != nil && autopot.runner != nil {
			autopot.runner.Stop()
			autopot.runner.Wait()
		}
		if timer != nil && timer.runner != nil {
			timer.runner.Stop()
			timer.runner.Wait()
		}
		if keyChain != nil && keyChain.runner != nil {
			keyChain.runner.Stop()
			keyChain.runner.Wait()
		}
		if session != nil {
			session.Reset()
		}
		a.mainWindow.Synchronize(func() {
			a.appendLog("Tools stopped — Start to relaunch")
			if a.overlay != nil {
				a.overlay.ShowStopped()
			}
		})
	}()
}
