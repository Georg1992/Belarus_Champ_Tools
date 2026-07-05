//go:build windows

package main

import (
	"fmt"
	"os"
	"runtime/debug"
	"strconv"

	"belarus-champ-tools/runner"
	"belarus-champ-tools/runner/profiles"
	"github.com/lxn/walk"
)

func (a *guiApp) buildAutoPotTab(page *walk.TabPage) error {
	layout := walk.NewVBoxLayout()
	layout.SetMargins(walk.Margins{HNear: 4, VNear: 4, HFar: 4, VFar: 4})
	layout.SetSpacing(10)
	if err := page.SetLayout(layout); err != nil {
		return err
	}

	if err := a.buildAutoPotModeSection(page); err != nil {
		return err
	}
	if err := a.buildHPPotionSection(page); err != nil {
		return err
	}
	if err := a.buildSPPotionSection(page); err != nil {
		return err
	}

	hintFont, err := walk.NewFont("Segoe UI", 8, 0)
	if err != nil {
		return err
	}
	hint, err := walk.NewLabel(page)
	if err != nil {
		return err
	}
	if err := hint.SetText("When HP or SP drops below the threshold, one potion is pressed at a time; the bar is polled until it recovers before using another."); err != nil {
		return err
	}
	hint.SetFont(hintFont)

	// Initial state: address controls disabled (default is Visual mode).
	a.setAutoPotAddressModeEnabled(false)
	return nil
}

// buildAutoPotModeSection creates the Detection mode group box with
// Visual/Address radio buttons, address controls, and wires their events.
func (a *guiApp) buildAutoPotModeSection(page *walk.TabPage) error {
	modeGB, err := walk.NewGroupBox(page)
	if err != nil {
		return err
	}
	if err := modeGB.SetTitle("Detection mode"); err != nil {
		return err
	}
	modeLayout := walk.NewVBoxLayout()
	modeLayout.SetSpacing(4)
	if err := modeGB.SetLayout(modeLayout); err != nil {
		return err
	}

	modeRow, err := walk.NewComposite(modeGB)
	if err != nil {
		return err
	}
	modeHBox := walk.NewHBoxLayout()
	modeHBox.SetSpacing(16)
	if err := modeRow.SetLayout(modeHBox); err != nil {
		return err
	}

	a.autopotVisualRB, err = walk.NewRadioButton(modeRow)
	if err != nil {
		return err
	}
	if err := a.autopotVisualRB.SetText("Visual (screen capture)"); err != nil {
		return err
	}
	a.autopotVisualRB.SetChecked(true)

	a.autopotAddressRB, err = walk.NewRadioButton(modeRow)
	if err != nil {
		return err
	}
	if err := a.autopotAddressRB.SetText("Address reading"); err != nil {
		return err
	}

	if err := a.buildAddressControls(modeGB); err != nil {
		return err
	}

	// Wire mode toggle.
	a.autopotVisualRB.CheckedChanged().Attach(func() {
		isAddress := a.autopotAddressRB.Checked()
		a.setAutoPotAddressModeEnabled(isAddress)
		if isAddress {
			a.appendLog("AutoPot mode: Address reading — select a game window and bind potion keys")
		}
	})
	a.autopotAddressRB.CheckedChanged().Attach(func() {
		isAddress := a.autopotAddressRB.Checked()
		a.setAutoPotAddressModeEnabled(isAddress)
	})
	return nil
}

// buildAddressControls creates the window selector, profile combo, refresh
// button, and wires their events (selection change, refresh click).
func (a *guiApp) buildAddressControls(modeGB *walk.GroupBox) error {
	addrRow, err := walk.NewComposite(modeGB)
	if err != nil {
		return err
	}
	addrHBox := walk.NewHBoxLayout()
	addrHBox.SetSpacing(8)
	if err := addrRow.SetLayout(addrHBox); err != nil {
		return err
	}

	winLabel, err := walk.NewLabel(addrRow)
	if err != nil {
		return err
	}
	if err := winLabel.SetText("Game window:"); err != nil {
		return err
	}

	a.windowCB, err = walk.NewComboBox(addrRow)
	if err != nil {
		return err
	}
	if err := a.windowCB.SetMinMaxSize(walk.Size{Width: 260, Height: 0}, walk.Size{Width: 260, Height: 0}); err != nil {
		return err
	}

	a.windowRefreshBtn, err = walk.NewPushButton(addrRow)
	if err != nil {
		return err
	}
	if err := a.windowRefreshBtn.SetText("Refresh"); err != nil {
		return err
	}

	profileLabel, err := walk.NewLabel(addrRow)
	if err != nil {
		return err
	}
	if err := profileLabel.SetText("Profile:"); err != nil {
		return err
	}

	a.profileCB, err = walk.NewComboBox(addrRow)
	if err != nil {
		return err
	}
	if err := a.profileCB.SetMinMaxSize(walk.Size{Width: 120, Height: 0}, walk.Size{Width: 120, Height: 0}); err != nil {
		return err
	}
	allProfiles := profiles.All()
	profileNames := make([]string, 0, len(allProfiles))
	for _, p := range allProfiles {
		profileNames = append(profileNames, p.Name)
	}
	if err := a.profileCB.SetModel(profileNames); err != nil {
		return err
	}
	if len(profileNames) > 0 {
		a.profileCB.SetCurrentIndex(0)
	}

	// Wire window selection.
	a.windowCB.CurrentIndexChanged().Attach(func() {
		if a.isRefreshingWindows {
			return
		}
		if a.windowCB.CurrentIndex() < 0 {
			a.processPID = 0
			a.clearAutoPotKeys()
			a.appendLog("Game window cleared — potion keys reset")
		} else if err := a.openSelectedProcessHandle(); err != nil {
			a.appendLog(fmt.Sprintf("Failed to open process: %v", err))
		} else {
			a.syncAutoPotSettings()
		}
	})

	// Wire refresh button.
	a.windowRefreshBtn.MouseDown().Attach(func(x, y int, button walk.MouseButton) {
		if button != walk.LeftButton {
			return
		}
		a.isRefreshingWindows = true
		windows, err := populateWindowComboBox(a.windowCB)
		if err != nil {
			a.appendLog(fmt.Sprintf("Window list refresh failed: %v", err))
		} else if len(windows) == 0 {
			a.appendLog("Window list refresh found no visible windows")
		}
		a.windowList = windows
		a.isRefreshingWindows = false
	})
	return nil
}

// potionSectionConfig carries the varying details for buildPotionSection.
type potionSectionConfig struct {
	title            string
	defaultThreshold int
	enabledCB        **walk.CheckBox
	thresholdEdit    **walk.LineEdit
	keyLabel         **walk.Label
	bindBtn          **walk.PushButton
	clearBtn         **walk.PushButton
	threshold        *int
	onBind           func()
	onClear          func()
	commitThresh     func()
}

// buildPotionSection creates a potion (HP/SP) group box with enable checkbox,
// threshold input, key label, bind button, and clear button. Two thin wrappers
// (buildHPPotionSection / buildSPPotionSection) call this with the right config.
func (a *guiApp) buildPotionSection(page *walk.TabPage, cfg potionSectionConfig) error {
	gb, err := walk.NewGroupBox(page)
	if err != nil {
		return err
	}
	if err := gb.SetTitle(cfg.title); err != nil {
		return err
	}
	layout := walk.NewHBoxLayout()
	layout.SetSpacing(10)
	if err := gb.SetLayout(layout); err != nil {
		return err
	}

	*cfg.enabledCB, err = walk.NewCheckBox(gb)
	if err != nil {
		return err
	}
	if err := (*cfg.enabledCB).SetText("Enabled"); err != nil {
		return err
	}
	(*cfg.enabledCB).SetChecked(true)
	(*cfg.enabledCB).CheckedChanged().Attach(a.syncAutoPotSettings)

	threshLabel, err := walk.NewLabel(gb)
	if err != nil {
		return err
	}
	if err := threshLabel.SetText("Trigger below %:"); err != nil {
		return err
	}

	*cfg.thresholdEdit, err = walk.NewLineEdit(gb)
	if err != nil {
		return err
	}
	(*cfg.thresholdEdit).SetMaxLength(2)
	if err := (*cfg.thresholdEdit).SetMinMaxSize(walk.Size{Width: 40, Height: 0}, walk.Size{Width: 40, Height: 0}); err != nil {
		return err
	}
	thresholdStr := strconv.Itoa(cfg.defaultThreshold)
	if err := (*cfg.thresholdEdit).SetText(thresholdStr); err != nil {
		return err
	}
	*cfg.threshold = cfg.defaultThreshold
	(*cfg.thresholdEdit).EditingFinished().Attach(func() {
		cfg.commitThresh()
		a.syncAutoPotSettings()
	})

	keyLabel, err := walk.NewLabel(gb)
	if err != nil {
		return err
	}
	if err := keyLabel.SetText("Key:"); err != nil {
		return err
	}

	*cfg.keyLabel, err = walk.NewLabel(gb)
	if err != nil {
		return err
	}
	if err := (*cfg.keyLabel).SetText("none"); err != nil {
		return err
	}

	*cfg.bindBtn, err = walk.NewPushButton(gb)
	if err != nil {
		return err
	}
	if err := (*cfg.bindBtn).SetText("Set key..."); err != nil {
		return err
	}
	(*cfg.bindBtn).Clicked().Attach(cfg.onBind)

	*cfg.clearBtn, err = walk.NewPushButton(gb)
	if err != nil {
		return err
	}
	if err := (*cfg.clearBtn).SetText("Clear"); err != nil {
		return err
	}
	(*cfg.clearBtn).Clicked().Attach(cfg.onClear)
	return nil
}

// buildHPPotionSection builds the HP potion group box using the shared builder.
func (a *guiApp) buildHPPotionSection(page *walk.TabPage) error {
	return a.buildPotionSection(page, potionSectionConfig{
		title:            "HP potion",
		defaultThreshold: 50,
		enabledCB:        &a.hpEnabledCB,
		thresholdEdit:    &a.hpThresholdEdit,
		keyLabel:         &a.hpKeyLabel,
		bindBtn:          &a.hpBindBtn,
		clearBtn:         &a.hpClearBtn,
		threshold:        &a.hpThreshold,
		onBind:           a.onBindHPKey,
		onClear:          a.onClearHPKey,
		commitThresh:     a.commitHPThresholdEdit,
	})
}

// buildSPPotionSection builds the SP potion group box using the shared builder.
func (a *guiApp) buildSPPotionSection(page *walk.TabPage) error {
	return a.buildPotionSection(page, potionSectionConfig{
		title:            "SP potion",
		defaultThreshold: 30,
		enabledCB:        &a.spEnabledCB,
		thresholdEdit:    &a.spThresholdEdit,
		keyLabel:         &a.spKeyLabel,
		bindBtn:          &a.spBindBtn,
		clearBtn:         &a.spClearBtn,
		threshold:        &a.spThreshold,
		onBind:           a.onBindSPKey,
		onClear:          a.onClearSPKey,
		commitThresh:     a.commitSPThresholdEdit,
	})
}

// isAutoPotAddressMode returns true if Address Reading mode is active.
func (a *guiApp) isAutoPotAddressMode() bool {
	return a.autopotAddressRB != nil && a.autopotAddressRB.Checked()
}

// setAutoPotAddressModeEnabled enables or disables the address-mode
// UI elements (window selector, profile). When switching away from
// address mode (back to Visual), clears the PID so the old handle
// isn't reused. Potion keys are preserved — they work for both modes.
func (a *guiApp) setAutoPotAddressModeEnabled(enabled bool) {
	a.windowCB.SetEnabled(enabled)
	a.windowRefreshBtn.SetEnabled(enabled)
	a.profileCB.SetEnabled(enabled)
	if !enabled {
		a.processPID = 0
	}
	a.syncAutoPotSettings()
}

// clearAutoPotKeys resets both HP and SP key bindings and logs it.
func (a *guiApp) clearAutoPotKeys() {
	a.hpKeyVK = 0
	a.spKeyVK = 0
	a.hpKeyLabel.SetText("none")
	a.spKeyLabel.SetText("none")
	a.syncAutoPotSettings()
}

// selectedProfile returns the server memory profile selected in the
// combo box, or the default profile if nothing is selected.
func (a *guiApp) selectedProfile() profiles.Profile {
	all := profiles.All()
	idx := a.profileCB.CurrentIndex()
	if idx >= 0 && idx < len(all) {
		return all[idx]
	}
	return profiles.Default()
}

// selectedWindowTitle returns the title of the currently selected window,
// or empty string if nothing is selected.
func (a *guiApp) selectedWindowTitle() string {
	idx := a.windowCB.CurrentIndex()
	if idx < 0 || idx >= len(a.windowList) {
		return ""
	}
	return a.windowList[idx].title
}

// openSelectedProcessHandle stores the selected window's PID for
// use by the address reader (which opens/closes handles per read).
func (a *guiApp) openSelectedProcessHandle() error {
	idx := a.windowCB.CurrentIndex()
	if idx < 0 || idx >= len(a.windowList) {
		return nil // nothing selected
	}

	win := a.windowList[idx]
	// Only log and update PID if it actually changed (guards against
	// spurious CurrentIndexChanged firings from Walk's combo box).
	if a.processPID != win.pid {
		a.processPID = win.pid
		a.appendLog(fmt.Sprintf("Selected %q (PID %d)", win.title, win.pid))
	}
	return nil
}

func (a *guiApp) autopotConfig() runner.AutoPotConfig {
	hpName := ""
	if a.hpKeyVK != 0 {
		hpName = runner.KeyName(a.hpKeyVK)
	}
	spName := ""
	if a.spKeyVK != 0 {
		spName = runner.KeyName(a.spKeyVK)
	}
	modeFn := func(mode string) {
		if a.overlay == nil {
			return
		}
		a.mainWindow.Synchronize(func() {
			a.overlay.SetMode(mode)
		})
	}

	statusFn := func(hp, hpMax, sp, spMax, stripX, stripY, stripW, stripH int) {
		if a.overlay == nil {
			return
		}
		a.mainWindow.Synchronize(func() {
			a.overlay.SetValues(hp, hpMax, sp, spMax)
			if stripW > 0 && stripH > 0 {
				a.overlay.SetPanelRect(stripX, stripY, stripW, stripH)
			}
		})
	}

	isAddress := a.isAutoPotAddressMode()
	profile := a.selectedProfile()
	cfg := runner.AutoPotConfig{
		Core: runner.CoreConfig{
			HPThreshold:    a.hpThreshold,
			SPThreshold:    a.spThreshold,
			HPKeyVK:        a.hpKeyVK,
			SPKeyVK:        a.spKeyVK,
			HPKeyName:      hpName,
			SPKeyName:      spName,
			HPEnabled:      a.hpEnabledCB.Checked(),
			SPEnabled:      a.spEnabledCB.Checked(),
			Log:            a.appendLog,
			OnStatusParsed: statusFn,
			OnStatusUIMode: modeFn,
		},
	}
	if isAddress && a.processPID != 0 {
		cfg.Address = &runner.AddressConfig{
			ProcessPID:   a.processPID,
			ProcessTitle: a.selectedWindowTitle(),
			Profile:      profile,
		}
	}
	return cfg
}

func (a *guiApp) commitHPThresholdEdit() {
	v, ok := a.parseThreshold(a.hpThresholdEdit)
	if !ok {
		a.hpThresholdEdit.SetText(strconv.Itoa(a.hpThreshold))
		return
	}
	if v == a.hpThreshold {
		return
	}
	a.hpThreshold = v
	a.appendLog(fmt.Sprintf("AutoPot HP threshold: %d%%", v))
}

func (a *guiApp) commitSPThresholdEdit() {
	v, ok := a.parseThreshold(a.spThresholdEdit)
	if !ok {
		a.spThresholdEdit.SetText(strconv.Itoa(a.spThreshold))
		return
	}
	if v == a.spThreshold {
		return
	}
	a.spThreshold = v
	a.appendLog(fmt.Sprintf("AutoPot SP threshold: %d%%", v))
}

func (a *guiApp) syncAutoPotSettings() {
	cfg := a.autopotWanted()
	a.mu.Lock()
	cfg.Core.Session = a.inputSession
	cfg.Core.Log = a.appendLog
	r := a.autopotRunner
	a.mu.Unlock()

	if cfg.Core.Session == nil || cfg.Core.Log == nil {
		return
	}

	if r != nil && r.Running() {
		// If neither HP nor SP keys are bound, stop the runner
		// instead of letting it spin doing nothing.
		if !cfg.Core.HPEnabled && !cfg.Core.SPEnabled {
			a.mu.Lock()
			a.autopotRunner = nil
			a.mu.Unlock()
			go func(old *runner.AutoPotRunner) {
				defer func() {
					if r := recover(); r != nil {
						_, _ = fmt.Fprintf(os.Stderr, "PANIC in autopot stop: %v\n%s\n", r, debug.Stack())
					}
				}()
				old.Stop()
				old.Wait()
			}(r)
			if a.overlay != nil {
				a.overlay.SetMode("AutoPot off")
			}
			return
		}

		// If AddressMode changed (Visual→Address or Address→Visual),
		// we must stop the runner and start a new one. The reader
		// (addressReader / statusUIReader / pixelBarReader) is created
		// once inside run() — UpdateSettings only changes the config,
		// it doesn't recreate the reader.
		if cfg.IsAddressMode() != a.prevAutoPotAddressMode {
			a.prevAutoPotAddressMode = cfg.IsAddressMode()
			a.mu.Lock()
			a.autopotRunner = nil
			a.mu.Unlock()
			// Stop synchronously (fast — just sets the stop flag) so
			// the new runner doesn't overlap with the old one on the
			// same InputSession (VIIPER connection).
			r.Stop()
			go func(old *runner.AutoPotRunner) {
				defer func() {
					if r := recover(); r != nil {
						_, _ = fmt.Fprintf(os.Stderr, "PANIC in autopot mode-switch wait: %v\n%s\n", r, debug.Stack())
					}
				}()
				old.Wait()
			}(r)
			a.startAutoPotRunner(cfg, a.guiLog(a.appendLog))
			return
		}

		r.UpdateSettings(cfg)
		return
	}

	if !a.isStarted() {
		return
	}

	a.prevAutoPotAddressMode = cfg.IsAddressMode()
	a.startAutoPotRunner(cfg, a.guiLog(a.appendLog))
}

func (a *guiApp) setAutoPotConfigEnabled(enabled bool) {
	a.hpEnabledCB.SetEnabled(enabled)
	a.spEnabledCB.SetEnabled(enabled)
	a.hpThresholdEdit.SetEnabled(enabled)
	a.spThresholdEdit.SetEnabled(enabled)
	a.hpBindBtn.SetEnabled(enabled)
	a.hpClearBtn.SetEnabled(enabled)
	a.spBindBtn.SetEnabled(enabled)
	a.spClearBtn.SetEnabled(enabled)
	a.autopotVisualRB.SetEnabled(enabled)
	a.autopotAddressRB.SetEnabled(enabled)
	// Address-mode sub-controls follow the mode+enabled state.
	isAddress := a.isAutoPotAddressMode()
	a.windowCB.SetEnabled(enabled && isAddress)
	a.windowRefreshBtn.SetEnabled(enabled && isAddress)
	a.profileCB.SetEnabled(enabled && isAddress)
}

func (a *guiApp) onClearHPKey() {
	a.hpKeyVK = 0
	a.hpKeyLabel.SetText("none")
	a.appendLog("HP potion key cleared")
	a.syncAutoPotSettings()
}

func (a *guiApp) onClearSPKey() {
	a.spKeyVK = 0
	a.spKeyLabel.SetText("none")
	a.appendLog("SP potion key cleared")
	a.syncAutoPotSettings()
}

func (a *guiApp) finishThresholdInput() {
	a.commitHPThresholdEdit()
	a.commitSPThresholdEdit()
	a.syncAutoPotSettings()
	a.blurThresholdEdits()
}

func (a *guiApp) wireThresholdBlurOnClick(container walk.Container) {
	if container == nil {
		return
	}
	children := container.Children()
	if children == nil {
		return
	}
	for i := 0; i < children.Len(); i++ {
		child := children.At(i)
		if child == a.hpThresholdEdit || child == a.spThresholdEdit || child == a.logList {
			continue
		}
		if win, ok := child.(walk.Window); ok {
			win.MouseDown().Attach(func(x, y int, button walk.MouseButton) {
				a.finishThresholdInput()
			})
		}
		if c, ok := child.(walk.Container); ok {
			a.wireThresholdBlurOnClick(c)
		}
	}
}

func (a *guiApp) blurThresholdEdits() {
	if a.mainWindow != nil {
		_ = a.mainWindow.SetFocus()
	}
}

func (a *guiApp) parseThreshold(edit *walk.LineEdit) (int, bool) {
	if edit == nil {
		return 0, false
	}
	v, err := strconv.Atoi(edit.Text())
	if err != nil || v < 1 || v > 99 {
		return 0, false
	}
	return v, true
}

func (a *guiApp) onBindHPKey() {
	a.bindAutoPotKey(true)
}

func (a *guiApp) onBindSPKey() {
	a.bindAutoPotKey(false)
}

func (a *guiApp) bindAutoPotKey(hp bool) {
	a.bindKeyFlow(
		func() bool {
			if !a.isViiperReady() || a.bindingActive {
				return false
			}
			if a.isAutoPotAddressMode() && a.windowCB.CurrentIndex() < 0 {
				a.appendLog("Cannot bind — select a game window first for Address Reading mode")
				return false
			}
			a.bindingActive = true
			if hp {
				a.hpBindBtn.SetEnabled(false)
			} else {
				a.spBindBtn.SetEnabled(false)
			}
			return true
		},
		fmt.Sprintf("Press a potion hotkey to assign (%s timeout)...", runner.KeyBindTimeout),
		func() { a.bindingActive = false },
		func() { a.setAutoPotConfigEnabled(a.isViiperReady()) },
		func(vk int32) {
			a.unsetKeyBinding(vk)
			if hp {
				a.hpKeyVK = vk
				a.hpKeyLabel.SetText(runner.KeyName(vk))
				a.appendLog(fmt.Sprintf("HP potion key: %s", runner.KeyName(vk)))
			} else {
				a.spKeyVK = vk
				a.spKeyLabel.SetText(runner.KeyName(vk))
				a.appendLog(fmt.Sprintf("SP potion key: %s", runner.KeyName(vk)))
			}
			a.syncAutoPotSettings()
		},
	)
}