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

// autopotTabController manages the AutoPot tab state and its runner.
type autopotTabController struct {
	ctx *tabContext

	// HP/SP widgets
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

	// Address mode widgets
	autopotVisualRB  *walk.RadioButton
	autopotAddressRB *walk.RadioButton
	windowCB         *walk.ComboBox
	windowRefreshBtn *walk.PushButton
	profileCB        *walk.ComboBox

	// State
	hpKeyVK              int32
	spKeyVK              int32
	hpThreshold          int
	spThreshold          int
	processPID           uint32
	windowList           []windowInfo
	isRefreshingWindows  bool
	prevAddressMode      bool

	runner *runner.AutoPotRunner
}

func newAutopotTabController(ctx *tabContext) *autopotTabController {
	return &autopotTabController{}
}

func (c *autopotTabController) runnerPtr() **runner.AutoPotRunner { return &c.runner }

func (c *autopotTabController) build(page *walk.TabPage) error {
	layout := walk.NewVBoxLayout()
	layout.SetMargins(walk.Margins{HNear: 4, VNear: 4, HFar: 4, VFar: 4})
	layout.SetSpacing(10)
	if err := page.SetLayout(layout); err != nil {
		return err
	}

	if err := c.buildModeSection(page); err != nil {
		return err
	}
	if err := c.buildHPPotionSection(page); err != nil {
		return err
	}
	if err := c.buildSPPotionSection(page); err != nil {
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

	c.setAddressModeEnabled(false)
	return nil
}

func (c *autopotTabController) buildModeSection(page *walk.TabPage) error {
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

	c.autopotVisualRB, err = walk.NewRadioButton(modeRow)
	if err != nil {
		return err
	}
	if err := c.autopotVisualRB.SetText("Visual (screen capture)"); err != nil {
		return err
	}
	c.autopotVisualRB.SetChecked(true)

	c.autopotAddressRB, err = walk.NewRadioButton(modeRow)
	if err != nil {
		return err
	}
	if err := c.autopotAddressRB.SetText("Address reading"); err != nil {
		return err
	}

	if err := c.buildAddressControls(modeGB); err != nil {
		return err
	}

	c.autopotVisualRB.CheckedChanged().Attach(func() {
		isAddress := c.autopotAddressRB.Checked()
		c.setAddressModeEnabled(isAddress)
		if isAddress {
			c.ctx.appendLog("AutoPot mode: Address reading — select a game window and bind potion keys")
		}
	})
	c.autopotAddressRB.CheckedChanged().Attach(func() {
		isAddress := c.autopotAddressRB.Checked()
		c.setAddressModeEnabled(isAddress)
	})
	return nil
}

func (c *autopotTabController) buildAddressControls(modeGB *walk.GroupBox) error {
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

	c.windowCB, err = walk.NewComboBox(addrRow)
	if err != nil {
		return err
	}
	if err := c.windowCB.SetMinMaxSize(walk.Size{Width: 260, Height: 0}, walk.Size{Width: 260, Height: 0}); err != nil {
		return err
	}

	c.windowRefreshBtn, err = walk.NewPushButton(addrRow)
	if err != nil {
		return err
	}
	if err := c.windowRefreshBtn.SetText("Refresh"); err != nil {
		return err
	}

	profileLabel, err := walk.NewLabel(addrRow)
	if err != nil {
		return err
	}
	if err := profileLabel.SetText("Profile:"); err != nil {
		return err
	}

	c.profileCB, err = walk.NewComboBox(addrRow)
	if err != nil {
		return err
	}
	if err := c.profileCB.SetMinMaxSize(walk.Size{Width: 120, Height: 0}, walk.Size{Width: 120, Height: 0}); err != nil {
		return err
	}
	allProfiles := profiles.All()
	profileNames := make([]string, 0, len(allProfiles))
	for _, p := range allProfiles {
		profileNames = append(profileNames, p.Name)
	}
	if err := c.profileCB.SetModel(profileNames); err != nil {
		return err
	}
	if len(profileNames) > 0 {
		c.profileCB.SetCurrentIndex(0)
	}

	c.windowCB.CurrentIndexChanged().Attach(func() {
		if c.isRefreshingWindows {
			return
		}
		if c.windowCB.CurrentIndex() < 0 {
			c.processPID = 0
			c.clearKeys()
			c.ctx.appendLog("Game window cleared — potion keys reset")
		} else if err := c.openSelectedProcess(); err != nil {
			c.ctx.appendLog(fmt.Sprintf("Failed to open process: %v", err))
		} else {
			c.syncSettings()
		}
	})

	c.windowRefreshBtn.MouseDown().Attach(func(x, y int, button walk.MouseButton) {
		if button != walk.LeftButton {
			return
		}
		c.isRefreshingWindows = true
		windows, err := populateWindowComboBox(c.windowCB)
		if err != nil {
			c.ctx.appendLog(fmt.Sprintf("Window list refresh failed: %v", err))
		} else if len(windows) == 0 {
			c.ctx.appendLog("Window list refresh found no visible windows")
		}
		c.windowList = windows
		c.isRefreshingWindows = false
	})
	return nil
}

func (c *autopotTabController) buildPotionSection(page *walk.TabPage, cfg potionSectionConfig) error {
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
	(*cfg.enabledCB).CheckedChanged().Attach(c.syncSettings)

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
		c.syncSettings()
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

func (c *autopotTabController) buildHPPotionSection(page *walk.TabPage) error {
	return c.buildPotionSection(page, potionSectionConfig{
		title:            "HP potion",
		defaultThreshold: 50,
		enabledCB:        &c.hpEnabledCB,
		thresholdEdit:    &c.hpThresholdEdit,
		keyLabel:         &c.hpKeyLabel,
		bindBtn:          &c.hpBindBtn,
		clearBtn:         &c.hpClearBtn,
		threshold:        &c.hpThreshold,
		onBind:           c.onBindHPKey,
		onClear:          c.onClearHPKey,
		commitThresh:     c.commitHPThreshold,
	})
}

func (c *autopotTabController) buildSPPotionSection(page *walk.TabPage) error {
	return c.buildPotionSection(page, potionSectionConfig{
		title:            "SP potion",
		defaultThreshold: 30,
		enabledCB:        &c.spEnabledCB,
		thresholdEdit:    &c.spThresholdEdit,
		keyLabel:         &c.spKeyLabel,
		bindBtn:          &c.spBindBtn,
		clearBtn:         &c.spClearBtn,
		threshold:        &c.spThreshold,
		onBind:           c.onBindSPKey,
		onClear:          c.onClearSPKey,
		commitThresh:     c.commitSPThreshold,
	})
}

func (c *autopotTabController) isAddressMode() bool {
	return c.autopotAddressRB != nil && c.autopotAddressRB.Checked()
}

func (c *autopotTabController) setAddressModeEnabled(enabled bool) {
	c.windowCB.SetEnabled(enabled)
	c.windowRefreshBtn.SetEnabled(enabled)
	c.profileCB.SetEnabled(enabled)
	if !enabled {
		c.processPID = 0
	}
	c.syncSettings()
}

func (c *autopotTabController) clearKeys() {
	c.hpKeyVK = 0
	c.spKeyVK = 0
	c.hpKeyLabel.SetText("none")
	c.spKeyLabel.SetText("none")
	c.syncSettings()
}

func (c *autopotTabController) selectedProfile() profiles.Profile {
	all := profiles.All()
	idx := c.profileCB.CurrentIndex()
	if idx >= 0 && idx < len(all) {
		return all[idx]
	}
	return profiles.Default()
}

func (c *autopotTabController) selectedWindowTitle() string {
	idx := c.windowCB.CurrentIndex()
	if idx < 0 || idx >= len(c.windowList) {
		return ""
	}
	return c.windowList[idx].title
}

func (c *autopotTabController) openSelectedProcess() error {
	idx := c.windowCB.CurrentIndex()
	if idx < 0 || idx >= len(c.windowList) {
		return nil
	}
	win := c.windowList[idx]
	if c.processPID != win.pid {
		c.processPID = win.pid
		c.ctx.appendLog(fmt.Sprintf("Selected %q (PID %d)", win.title, win.pid))
	}
	return nil
}

func (c *autopotTabController) autopotConfig() runner.AutoPotConfig {
	hpName := ""
	if c.hpKeyVK != 0 {
		hpName = runner.KeyName(c.hpKeyVK)
	}
	spName := ""
	if c.spKeyVK != 0 {
		spName = runner.KeyName(c.spKeyVK)
	}
	modeFn := func(mode string) {
		if c.ctx.overlay == nil {
			return
		}
		c.ctx.window.Synchronize(func() { c.ctx.overlay.SetMode(mode) })
	}
	statusFn := func(hp, hpMax, sp, spMax, stripX, stripY, stripW, stripH int) {
		if c.ctx.overlay == nil {
			return
		}
		c.ctx.window.Synchronize(func() {
			c.ctx.overlay.SetValues(hp, hpMax, sp, spMax)
			if stripW > 0 && stripH > 0 {
				c.ctx.overlay.SetPanelRect(stripX, stripY, stripW, stripH)
			}
		})
	}

	isAddress := c.isAddressMode()
	profile := c.selectedProfile()
	cfg := runner.AutoPotConfig{
		Core: runner.CoreConfig{
			HPThreshold:    c.hpThreshold,
			SPThreshold:    c.spThreshold,
			HPKeyVK:        c.hpKeyVK,
			SPKeyVK:        c.spKeyVK,
			HPKeyName:      hpName,
			SPKeyName:      spName,
			HPEnabled:      c.hpEnabledCB.Checked(),
			SPEnabled:      c.spEnabledCB.Checked(),
			Log:            c.ctx.appendLog,
			OnStatusParsed: statusFn,
			OnStatusUIMode: modeFn,
		},
	}
	if isAddress && c.processPID != 0 {
		cfg.Address = &runner.AddressConfig{
			ProcessPID:   c.processPID,
			ProcessTitle: c.selectedWindowTitle(),
			Profile:      profile,
		}
	}
	return cfg
}

func (c *autopotTabController) wanted() runner.AutoPotConfig {
	cfg := c.autopotConfig()
	cfg.Core.HPEnabled = cfg.Core.HPEnabled && cfg.Core.HPKeyVK != 0
	cfg.Core.SPEnabled = cfg.Core.SPEnabled && cfg.Core.SPKeyVK != 0
	return cfg
}

func (c *autopotTabController) commitHPThreshold() {
	v, ok := c.parseThreshold(c.hpThresholdEdit)
	if !ok {
		c.hpThresholdEdit.SetText(strconv.Itoa(c.hpThreshold))
		return
	}
	if v == c.hpThreshold {
		return
	}
	c.hpThreshold = v
	c.ctx.appendLog(fmt.Sprintf("AutoPot HP threshold: %d%%", v))
}

func (c *autopotTabController) commitSPThreshold() {
	v, ok := c.parseThreshold(c.spThresholdEdit)
	if !ok {
		c.spThresholdEdit.SetText(strconv.Itoa(c.spThreshold))
		return
	}
	if v == c.spThreshold {
		return
	}
	c.spThreshold = v
	c.ctx.appendLog(fmt.Sprintf("AutoPot SP threshold: %d%%", v))
}

func (c *autopotTabController) parseThreshold(edit *walk.LineEdit) (int, bool) {
	if edit == nil {
		return 0, false
	}
	v, err := strconv.Atoi(edit.Text())
	if err != nil || v < 1 || v > 99 {
		return 0, false
	}
	return v, true
}

func (c *autopotTabController) syncSettings() {
	cfg := c.wanted()
	c.ctx.mu.Lock()
	cfg.Core.Session = c.ctx.session()
	cfg.Core.Log = c.ctx.appendLog
	r := c.runner
	c.ctx.mu.Unlock()

	if cfg.Core.Session == nil || cfg.Core.Log == nil {
		return
	}

	if r != nil && r.Running() {
		if !cfg.Core.HPEnabled && !cfg.Core.SPEnabled {
			c.ctx.mu.Lock()
			c.runner = nil
			c.ctx.mu.Unlock()
			go func(old *runner.AutoPotRunner) {
				defer func() {
					if r := recover(); r != nil {
						_, _ = fmt.Fprintf(os.Stderr, "PANIC in autopot stop: %v\n%s\n", r, debug.Stack())
					}
				}()
				old.Stop()
				old.Wait()
			}(r)
			if c.ctx.overlay != nil {
				c.ctx.overlay.SetMode("AutoPot off")
			}
			return
		}

		if cfg.IsAddressMode() != c.prevAddressMode {
			c.prevAddressMode = cfg.IsAddressMode()
			c.ctx.mu.Lock()
			c.runner = nil
			c.ctx.mu.Unlock()
			r.Stop()
			go func(old *runner.AutoPotRunner) {
				defer func() {
					if r := recover(); r != nil {
						_, _ = fmt.Fprintf(os.Stderr, "PANIC in autopot mode-switch wait: %v\n%s\n", r, debug.Stack())
					}
				}()
				old.Wait()
			}(r)
			c.startRunner(cfg, c.ctx.guiLog(c.ctx.appendLog))
			return
		}

		r.UpdateSettings(cfg)
		return
	}

	if !c.ctx.isStarted() {
		return
	}

	c.prevAddressMode = cfg.IsAddressMode()
	c.startRunner(cfg, c.ctx.guiLog(c.ctx.appendLog))
}

func (c *autopotTabController) setEnabled(enabled bool) {
	c.hpEnabledCB.SetEnabled(enabled)
	c.spEnabledCB.SetEnabled(enabled)
	c.hpThresholdEdit.SetEnabled(enabled)
	c.spThresholdEdit.SetEnabled(enabled)
	c.hpBindBtn.SetEnabled(enabled)
	c.hpClearBtn.SetEnabled(enabled)
	c.spBindBtn.SetEnabled(enabled)
	c.spClearBtn.SetEnabled(enabled)
	c.autopotVisualRB.SetEnabled(enabled)
	c.autopotAddressRB.SetEnabled(enabled)
	isAddress := c.isAddressMode()
	c.windowCB.SetEnabled(enabled && isAddress)
	c.windowRefreshBtn.SetEnabled(enabled && isAddress)
	c.profileCB.SetEnabled(enabled && isAddress)
}

func (c *autopotTabController) startRunner(cfg runner.AutoPotConfig, log func(string)) {
	take, store := makeLifecycleSlot[*runner.AutoPotRunner](c.ctx.mu, c.runnerPtr())
	startLifecycle(
		take, store,
		"AutoPot", log,
		func() runner.InputSession { return c.ctx.session() },
		func() bool { return cfg.Core.HPEnabled || cfg.Core.SPEnabled },
		func(sess runner.InputSession) *runner.AutoPotRunner {
			cfg.Core.Session = sess
			cfg.Core.Log = log
			return runner.NewAutoPot(cfg)
		},
	)
}

func (c *autopotTabController) onClearHPKey() {
	c.hpKeyVK = 0
	c.hpKeyLabel.SetText("none")
	c.ctx.appendLog("HP potion key cleared")
	c.syncSettings()
}

func (c *autopotTabController) onClearSPKey() {
	c.spKeyVK = 0
	c.spKeyLabel.SetText("none")
	c.ctx.appendLog("SP potion key cleared")
	c.syncSettings()
}

func (c *autopotTabController) onBindHPKey() { c.bindKey(true) }
func (c *autopotTabController) onBindSPKey() { c.bindKey(false) }

func (c *autopotTabController) bindKey(hp bool) {
	c.ctx.bindKeyFlow(
		func() bool {
			if !c.ctx.isViiperReady() || *c.ctx.bindActive {
				return false
			}
			if c.isAddressMode() && c.windowCB.CurrentIndex() < 0 {
				c.ctx.appendLog("Cannot bind — select a game window first for Address Reading mode")
				return false
			}
			*c.ctx.bindActive = true
			if hp {
				c.hpBindBtn.SetEnabled(false)
			} else {
				c.spBindBtn.SetEnabled(false)
			}
			return true
		},
		fmt.Sprintf("Press a potion hotkey to assign (%s timeout)...", runner.KeyBindTimeout),
		func() { *c.ctx.bindActive = false },
		func() { c.setEnabled(c.ctx.isViiperReady()) },
		func(vk int32) {
			c.ctx.unsetBinding(vk)
			if hp {
				c.hpKeyVK = vk
				c.hpKeyLabel.SetText(runner.KeyName(vk))
				c.ctx.appendLog(fmt.Sprintf("HP potion key: %s", runner.KeyName(vk)))
			} else {
				c.spKeyVK = vk
				c.spKeyLabel.SetText(runner.KeyName(vk))
				c.ctx.appendLog(fmt.Sprintf("SP potion key: %s", runner.KeyName(vk)))
			}
			c.syncSettings()
		},
	)
}

// unsetBinding removes vk from this controller. Returns true if removed.
func (c *autopotTabController) unsetBinding(vk int32) bool {
	if c.hpKeyVK == vk {
		c.hpKeyVK = 0
		c.hpKeyLabel.SetText("none")
		c.ctx.appendLog(fmt.Sprintf("Key %s removed from HP potion (reassigned)", runner.KeyName(vk)))
		c.syncSettings()
		return true
	}
	if c.spKeyVK == vk {
		c.spKeyVK = 0
		c.spKeyLabel.SetText("none")
		c.ctx.appendLog(fmt.Sprintf("Key %s removed from SP potion (reassigned)", runner.KeyName(vk)))
		c.syncSettings()
		return true
	}
	return false
}

// finishThresholdInput commits both threshold edits and syncs.
func (c *autopotTabController) finishThresholdInput() {
	c.commitHPThreshold()
	c.commitSPThreshold()
	c.syncSettings()
	if c.ctx.window != nil {
		_ = c.ctx.window.SetFocus()
	}
}

// wireThresholdBlurOnClick recursively attaches threshold blur to all widgets.
func (c *autopotTabController) wireThresholdBlurOnClick(container walk.Container, logList *walk.ListBox) {
	if container == nil {
		return
	}
	children := container.Children()
	if children == nil {
		return
	}
	for i := 0; i < children.Len(); i++ {
		child := children.At(i)
		if child == c.hpThresholdEdit || child == c.spThresholdEdit || child == logList {
			continue
		}
		if win, ok := child.(walk.Window); ok {
			win.MouseDown().Attach(func(x, y int, button walk.MouseButton) {
				c.finishThresholdInput()
			})
		}
		if cont, ok := child.(walk.Container); ok {
			c.wireThresholdBlurOnClick(cont, logList)
		}
	}
}
