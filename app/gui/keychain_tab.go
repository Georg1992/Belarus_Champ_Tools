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

// keyChainTabController manages the KeyChain tab state and its runner.
type keyChainTabController struct {
	ctx *tabContext

	slots       [runner.KeyChainSlotCount]keyChainSlotWidgets
	keyVKs      [runner.KeyChainSlotCount]int32
	clearBtn    *walk.PushButton
	bindingSlot int

	runner *runner.KeyChainRunner
}

func newKeyChainTabController(ctx *tabContext) *keyChainTabController {
	return &keyChainTabController{
		ctx:         ctx,
		bindingSlot: -1,
	}
}

func (c *keyChainTabController) runnerPtr() **runner.KeyChainRunner { return &c.runner }

func (c *keyChainTabController) build(page *walk.TabPage) error {
	layout := walk.NewVBoxLayout()
	layout.SetMargins(walk.Margins{HNear: 4, VNear: 4, HFar: 4, VFar: 4})
	layout.SetSpacing(10)
	if err := page.SetLayout(layout); err != nil {
		return err
	}

	if err := c.buildGroup(page); err != nil {
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

func (c *keyChainTabController) buildGroup(page *walk.TabPage) error {
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

	if err := c.buildLabels(chainRow); err != nil {
		return err
	}
	if err := c.buildSteps(chainRow); err != nil {
		return err
	}

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

	c.clearBtn, err = walk.NewPushButton(btnRow)
	if err != nil {
		return err
	}
	if err := c.clearBtn.SetText("Clear"); err != nil {
		return err
	}
	c.clearBtn.Clicked().Attach(c.clearAll)
	return nil
}

func (c *keyChainTabController) buildLabels(parent walk.Container) error {
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

func (c *keyChainTabController) buildSteps(parent walk.Container) error {
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
		if err := c.buildStep(stepsRow, i, stepHeight); err != nil {
			return err
		}
		if i < runner.KeyChainSlotCount-1 {
			if err := c.buildStepLink(stepsRow, stepHeight); err != nil {
				return err
			}
		}
	}
	return nil
}

func (c *keyChainTabController) buildStep(parent walk.Container, index, height int) error {
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

	w := &c.slots[index]
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
	c.setKeyText(index, 0)
	slot := index
	w.keyEdit.MouseDown().Attach(func(_ int, _ int, button walk.MouseButton) {
		if button == walk.LeftButton {
			c.bindKey(slot)
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
	w.delayEdit.ValueChanged().Attach(c.syncSettings)

	return nil
}

func (c *keyChainTabController) buildStepLink(parent walk.Container, height int) error {
	link, err := newKeyChainStepLink(parent)
	if err != nil {
		return err
	}
	return link.SetMinMaxSize(
		walk.Size{Width: keyChainLinkWidth, Height: height},
		walk.Size{Width: keyChainLinkWidth, Height: height},
	)
}

func (c *keyChainTabController) setKeyText(index int, vk int32) {
	text := "None"
	if vk != 0 {
		text = runner.KeyName(vk)
	}
	c.slots[index].keyEdit.SetText(text)
}

func (c *keyChainTabController) config() runner.KeyChainConfig {
	cfg := runner.KeyChainConfig{Log: c.ctx.appendLog}
	for i := 0; i < runner.KeyChainSlotCount; i++ {
		cfg.Keys[i] = c.keyVKs[i]
		cfg.DelaysMs[i] = int(c.slots[i].delayEdit.Value())
	}
	return cfg
}

func (c *keyChainTabController) syncSettings() {
	if !c.ctx.isStarted() {
		return
	}

	cfg := c.config()
	c.ctx.mu.Lock()
	kc := c.runner
	c.ctx.mu.Unlock()

	if !cfg.Active() {
		c.stopRunner()
		return
	}

	c.ctx.mu.Lock()
	cfg.Session = c.ctx.session()
	c.ctx.mu.Unlock()

	if kc != nil && kc.Running() {
		kc.UpdateSettings(cfg)
		return
	}

	c.startRunner(cfg, c.ctx.guiLog(c.ctx.appendLog))
}

func (c *keyChainTabController) setEnabled(enabled bool) {
	for i := 0; i < runner.KeyChainSlotCount; i++ {
		c.slots[i].keyEdit.SetEnabled(enabled)
		c.slots[i].delayEdit.SetEnabled(enabled)
	}
	if c.clearBtn != nil {
		c.clearBtn.SetEnabled(enabled)
	}
}

func (c *keyChainTabController) startRunner(cfg runner.KeyChainConfig, log func(string)) {
	take, store := makeLifecycleSlot[*runner.KeyChainRunner](c.ctx.mu, c.runnerPtr())
	startLifecycle(
		take, store,
		"KeyChain", log,
		func() runner.InputSession { return c.ctx.session() },
		func() bool { return cfg.Active() },
		func(sess runner.InputSession) *runner.KeyChainRunner {
			cfg.Session = sess
			cfg.Log = log
			return runner.NewKeyChain(cfg)
		},
	)
}

func (c *keyChainTabController) stopRunner() {
	c.ctx.mu.Lock()
	kc := c.runner
	c.runner = nil
	c.ctx.mu.Unlock()
	if kc != nil {
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

func (c *keyChainTabController) clearAll() {
	for i := 0; i < runner.KeyChainSlotCount; i++ {
		c.keyVKs[i] = 0
		c.setKeyText(i, 0)
		c.slots[i].delayEdit.SetValue(0)
	}
	c.syncSettings()
	c.ctx.appendLog("KeyChain cleared")
}

func (c *keyChainTabController) bindKey(index int) {
	c.ctx.bindKeyFlow(
		func() bool {
			if !c.ctx.isViiperReady() || *c.ctx.bindActive || index < 0 || index >= runner.KeyChainSlotCount {
				return false
			}
			*c.ctx.bindActive = true
			c.bindingSlot = index
			c.slots[index].keyEdit.SetEnabled(false)
			return true
		},
		fmt.Sprintf("Press a key for chain slot %d (%s timeout)...", index+1, runner.KeyBindTimeout),
		func() { c.bindingSlot = -1; *c.ctx.bindActive = false },
		func() { c.setEnabled(c.ctx.isViiperReady()) },
		func(vk int32) {
			c.ctx.unsetBinding(vk)
			c.keyVKs[index] = vk
			c.setKeyText(index, vk)
			c.ctx.appendLog(fmt.Sprintf("Chain key %d: %s", index+1, runner.KeyName(vk)))
			c.syncSettings()
		},
	)
}

// unsetBinding removes vk from this controller. Returns true if removed.
func (c *keyChainTabController) unsetBinding(vk int32) bool {
	for i := 0; i < runner.KeyChainSlotCount; i++ {
		if c.keyVKs[i] == vk {
			c.keyVKs[i] = 0
			c.setKeyText(i, 0)
			c.ctx.appendLog(fmt.Sprintf("Key %s removed from Chain slot %d (reassigned)", runner.KeyName(vk), i+1))
			c.syncSettings()
			return true
		}
	}
	return false
}
