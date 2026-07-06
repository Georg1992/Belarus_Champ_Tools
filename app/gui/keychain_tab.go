//go:build windows

package main

import (
	"fmt"
	"os"
	"runtime/debug"

	"belarus-champ-tools/runner"
	"github.com/lxn/walk"
)

type keyChainSlotWidgets struct {
	keyEdit   *walk.LineEdit
	delayEdit *walk.NumberEdit
}

type keychainController struct {
	slots       [runner.KeyChainSlotCount]keyChainSlotWidgets
	keyVKs      [runner.KeyChainSlotCount]int32
	clearBtn    *walk.PushButton
	bindingSlot int
}

func (c *keychainController) setKeyText(index int, vk int32) {
	text := "None"
	if vk != 0 {
		text = runner.KeyName(vk)
	}
	c.slots[index].keyEdit.SetText(text)
}

func (c *keychainController) config(logFn func(string)) runner.KeyChainConfig {
	cfg := runner.KeyChainConfig{Log: logFn}
	for i := 0; i < runner.KeyChainSlotCount; i++ {
		cfg.Keys[i] = c.keyVKs[i]
		cfg.DelaysMs[i] = int(c.slots[i].delayEdit.Value())
	}
	return cfg
}

func (a *guiApp) buildKeyChainTab(page *walk.TabPage) error {
	layout := walk.NewVBoxLayout()
	layout.SetMargins(walk.Margins{HNear: 4, VNear: 4, HFar: 4, VFar: 4})
	layout.SetSpacing(10)
	if err := page.SetLayout(layout); err != nil {
		return err
	}

	if err := a.buildKeyChainGroup(page); err != nil {
		return err
	}

	hint, err := walk.NewLabel(page)
	if err != nil {
		return err
	}
	if err := hint.SetText("Key 1 is the trigger. Tap it to run the chain once; hold it to loop."); err != nil {
		return err
	}
	return nil
}

// buildKeyChainGroup creates the Switch 1 group box with labels, step columns,
// step links, and the Clear button.
func (a *guiApp) buildKeyChainGroup(page *walk.TabPage) error {
	chainGB, err := walk.NewGroupBox(page)
	if err != nil {
		return err
	}
	if err := chainGB.SetTitle("Switch 1"); err != nil {
		return err
	}
	chainLayout := walk.NewVBoxLayout()
	chainLayout.SetSpacing(10)
	if err := chainGB.SetLayout(chainLayout); err != nil {
		return err
	}

	// Main row: labels | steps
	chainRow, err := walk.NewComposite(chainGB)
	if err != nil {
		return err
	}
	chainHBox := walk.NewHBoxLayout()
	chainHBox.SetSpacing(0)
	if err := chainRow.SetLayout(chainHBox); err != nil {
		return err
	}
	applyKeyChainSurface(chainRow)

	if err := a.buildKeyChainLabels(chainRow); err != nil {
		return err
	}
	if err := a.buildKeyChainSteps(chainRow); err != nil {
		return err
	}

	// Clear button row
	btnRow, err := walk.NewComposite(chainGB)
	if err != nil {
		return err
	}
	btnLayout := walk.NewHBoxLayout()
	btnLayout.SetSpacing(10)
	if err := btnRow.SetLayout(btnLayout); err != nil {
		return err
	}
	applyKeyChainSurface(btnRow)

	a.keychain.clearBtn, err = walk.NewPushButton(btnRow)
	if err != nil {
		return err
	}
	if err := a.keychain.clearBtn.SetText("Clear"); err != nil {
		return err
	}
	a.keychain.clearBtn.Clicked().Attach(a.clearKeyChain)
	return nil
}

// buildKeyChainLabels creates the Keys/Delay(ms) label column.
func (a *guiApp) buildKeyChainLabels(parent walk.Container) error {
	labelsCol, err := walk.NewComposite(parent)
	if err != nil {
		return err
	}
	labelsLayout := walk.NewVBoxLayout()
	labelsLayout.SetSpacing(0)
	if err := labelsCol.SetLayout(labelsLayout); err != nil {
		return err
	}
	if err := labelsCol.SetMinMaxSize(walk.Size{Width: 70, Height: 0}, walk.Size{Width: 70, Height: 0}); err != nil {
		return err
	}
	applyKeyChainSurface(labelsCol)

	keysLabel, err := walk.NewLabel(labelsCol)
	if err != nil {
		return err
	}
	if err := keysLabel.SetText("Keys:"); err != nil {
		return err
	}
	if err := keysLabel.SetMinMaxSize(walk.Size{Width: 70, Height: keyChainFieldHeight}, walk.Size{Width: 70, Height: keyChainFieldHeight}); err != nil {
		return err
	}

	downSpacer, err := walk.NewComposite(labelsCol)
	if err != nil {
		return err
	}
	if err := downSpacer.SetMinMaxSize(walk.Size{Width: 70, Height: keyChainDownHeight}, walk.Size{Width: 70, Height: keyChainDownHeight}); err != nil {
		return err
	}
	applyKeyChainSurface(downSpacer)

	delaysLabel, err := walk.NewLabel(labelsCol)
	if err != nil {
		return err
	}
	if err := delaysLabel.SetText("Delay(ms):"); err != nil {
		return err
	}
	if err := delaysLabel.SetMinMaxSize(walk.Size{Width: 70, Height: keyChainFieldHeight}, walk.Size{Width: 70, Height: keyChainFieldHeight}); err != nil {
		return err
	}
	return nil
}

// buildKeyChainSteps creates the step column with all 7 key slots and links.
func (a *guiApp) buildKeyChainSteps(parent walk.Container) error {
	stepsRow, err := walk.NewComposite(parent)
	if err != nil {
		return err
	}
	stepsHBox := walk.NewHBoxLayout()
	stepsHBox.SetSpacing(0)
	if err := stepsRow.SetLayout(stepsHBox); err != nil {
		return err
	}
	applyKeyChainSurface(stepsRow)

	stepHeight := keyChainStepHeight()
	for i := 0; i < runner.KeyChainSlotCount; i++ {
		if err := a.buildKeyChainStep(stepsRow, i, stepHeight); err != nil {
			return err
		}
		if i < runner.KeyChainSlotCount-1 {
			if err := a.buildKeyChainStepLink(stepsRow, stepHeight); err != nil {
				return err
			}
		}
	}
	return nil
}

func (a *guiApp) buildKeyChainStep(parent walk.Container, index, height int) error {
	step, err := walk.NewComposite(parent)
	if err != nil {
		return err
	}
	stepLayout := walk.NewVBoxLayout()
	stepLayout.SetSpacing(0)
	if err := step.SetLayout(stepLayout); err != nil {
		return err
	}
	if err := step.SetMinMaxSize(walk.Size{Width: keyChainStepWidth, Height: height}, walk.Size{Width: keyChainStepWidth, Height: height}); err != nil {
		return err
	}
	applyKeyChainSurface(step)

	w := &a.keychain.slots[index]
	w.keyEdit, err = walk.NewLineEdit(step)
	if err != nil {
		return err
	}
	if err := w.keyEdit.SetReadOnly(true); err != nil {
		return err
	}
	if err := w.keyEdit.SetMinMaxSize(walk.Size{Width: keyChainKeyFieldWidth, Height: keyChainFieldHeight}, walk.Size{Width: keyChainKeyFieldWidth, Height: keyChainFieldHeight}); err != nil {
		return err
	}
	a.keychain.setKeyText(index, 0)
	slot := index
	w.keyEdit.MouseDown().Attach(func(_ int, _ int, button walk.MouseButton) {
		if button == walk.LeftButton {
			a.bindKeyChainKey(slot)
		}
	})

	downArrow, err := newKeyChainDownArrow(step)
	if err != nil {
		return err
	}
	if err := downArrow.SetMinMaxSize(walk.Size{Width: keyChainStepWidth, Height: keyChainDownHeight}, walk.Size{Width: keyChainStepWidth, Height: keyChainDownHeight}); err != nil {
		return err
	}

	w.delayEdit, err = walk.NewNumberEdit(step)
	if err != nil {
		return err
	}
	if err := w.delayEdit.SetRange(0, 999999); err != nil {
		return err
	}
	if err := w.delayEdit.SetDecimals(0); err != nil {
		return err
	}
	if err := w.delayEdit.SetIncrement(1); err != nil {
		return err
	}
	if err := w.delayEdit.SetSpinButtonsVisible(true); err != nil {
		return err
	}
	if err := w.delayEdit.SetValue(0); err != nil {
		return err
	}
	if err := w.delayEdit.SetMinMaxSize(walk.Size{Width: keyChainDelayFieldWidth, Height: keyChainFieldHeight}, walk.Size{Width: keyChainDelayFieldWidth, Height: keyChainFieldHeight}); err != nil {
		return err
	}
	w.delayEdit.ValueChanged().Attach(a.syncKeyChainSettings)

	return nil
}

func (a *guiApp) buildKeyChainStepLink(parent walk.Container, height int) error {
	link, err := newKeyChainStepLink(parent)
	if err != nil {
		return err
	}
	return link.SetMinMaxSize(
		walk.Size{Width: keyChainLinkWidth, Height: height},
		walk.Size{Width: keyChainLinkWidth, Height: height},
	)
}

func (a *guiApp) syncKeyChainSettings() {
	if !a.isStarted() {
		return
	}

	cfg :=	a.keychain.config(a.appendLog)
	a.mu.Lock()
	kc := a.keychainRunner
	a.mu.Unlock()

	if !cfg.Active() {
		a.stopKeyChainRunner()
		return
	}

	a.mu.Lock()
	cfg.Session = a.inputSession
	a.mu.Unlock()

	if kc != nil && kc.Running() {
		kc.UpdateSettings(cfg)
		return
	}

	a.startKeyChainRunner(cfg, a.guiLog(a.appendLog))
}

func (a *guiApp) setKeyChainConfigEnabled(enabled bool) {
	for i := 0; i < runner.KeyChainSlotCount; i++ {
		a.keychain.slots[i].keyEdit.SetEnabled(enabled)
		a.keychain.slots[i].delayEdit.SetEnabled(enabled)
	}
	if a.keychain.clearBtn != nil {
		a.keychain.clearBtn.SetEnabled(enabled)
	}
}

func (a *guiApp) startKeyChainRunner(cfg runner.KeyChainConfig, log func(string)) {
	take, store := makeLifecycleSlot[*runner.KeyChainRunner](&a.mu, &a.keychainRunner)
	startLifecycle(
		take, store,
		"KeyChain",
		log,
		func() runner.InputSession {
			a.mu.Lock()
			defer a.mu.Unlock()
			return a.inputSession
		},
		func() bool { return cfg.Active() },
		func(sess runner.InputSession) *runner.KeyChainRunner {
			cfg.Session = sess
			cfg.Log = log
			return runner.NewKeyChain(cfg)
		},
	)
}

func (a *guiApp) stopKeyChainRunner() {
	a.mu.Lock()
	kc := a.keychainRunner
	a.keychainRunner = nil
	a.mu.Unlock()
	if kc != nil {
		// Stop+Wait on a background goroutine to avoid
		// deadlocking the GUI thread if the runner
		// goroutine is in a Synchronize call.
		go func(old *runner.KeyChainRunner) {
			defer func() {
				if r := recover(); r != nil {
					_, _ = fmt.Fprintf(os.Stderr, "PANIC in keyChain stop: %v\n%s\n", r, debug.Stack())
				}
			}()
			old.Stop()
			old.Wait()
		}(kc)
	}
}

func (a *guiApp) clearKeyChain() {
	for i := 0; i < runner.KeyChainSlotCount; i++ {
		a.keychain.keyVKs[i] = 0
		a.keychain.setKeyText(i, 0)
		a.keychain.slots[i].delayEdit.SetValue(0)
	}
	a.syncKeyChainSettings()
	a.appendLog("KeyChain cleared")
}

func (a *guiApp) bindKeyChainKey(index int) {
	a.bindKeyFlow(
		func() bool {
			if !a.isViiperReady() || a.bindingActive || index < 0 || index >= runner.KeyChainSlotCount {
				return false
			}
			a.bindingActive = true
			a.keychain.bindingSlot = index
			a.keychain.slots[index].keyEdit.SetEnabled(false)
			return true
		},
		fmt.Sprintf("Press a key for chain slot %d (%s timeout)...", index+1, runner.KeyBindTimeout),
		func() { a.keychain.bindingSlot = -1; a.bindingActive = false },
		func() { a.setKeyChainConfigEnabled(a.isViiperReady()) },
		func(vk int32) {
			a.unsetKeyBinding(vk)
			a.keychain.keyVKs[index] = vk
			a.keychain.setKeyText(index, vk)
			a.appendLog(fmt.Sprintf("Chain key %d: %s", index+1, runner.KeyName(vk)))
			a.syncKeyChainSettings()
		},
	)
}
