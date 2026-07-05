//go:build windows

package main

import (
	"fmt"
	"strconv"

	"belarus-champ-tools/runner"
	"github.com/lxn/walk"
)

const (
	clickerWithMouse    = 0
	clickerWithoutMouse = 1
)

var clickerSlotTitles = [runner.ClickerSlotCount]string{
	"With mouse click",
	"Without mouse click (keyboard only)",
}

type clickerSlotWidgets struct {
	keyLabel  *walk.Label
	bindBtn   *walk.PushButton
	clearBtn  *walk.PushButton
	delayEdit *walk.LineEdit
}

// clickerTabController manages the Clicker tab state and its runner.
type clickerTabController struct {
	ctx *tabContext

	slots           [runner.ClickerSlotCount]clickerSlotWidgets
	triggerVKs      [runner.ClickerSlotCount][]int32
	bindingSlot     int
	lastLoggedDelay [runner.ClickerSlotCount]int

	runner *runner.Runner
}

func newClickerTabController(ctx *tabContext) *clickerTabController {
	return &clickerTabController{
		ctx:         ctx,
		bindingSlot: -1,
	}
}

// runnerPtr returns a pointer to the runner field for makeLifecycleSlot.
func (c *clickerTabController) runnerPtr() **runner.Runner { return &c.runner }

func (c *clickerTabController) build(page *walk.TabPage, timer *timerKeyTabController) error {
	layout := walk.NewVBoxLayout()
	layout.SetMargins(walk.Margins{HNear: 4, VNear: 4, HFar: 4, VFar: 4})
	layout.SetSpacing(10)
	if err := page.SetLayout(layout); err != nil {
		return err
	}

	configGB, err := walk.NewGroupBox(page)
	if err != nil {
		return err
	}
	if err := configGB.SetTitle("2. Configure clickers"); err != nil {
		return err
	}
	configLayout := walk.NewVBoxLayout()
	configLayout.SetSpacing(10)
	if err := configGB.SetLayout(configLayout); err != nil {
		return err
	}

	for i := 0; i < runner.ClickerSlotCount; i++ {
		if err := c.buildSlot(configGB, i); err != nil {
			return err
		}
	}

	configHint, err := walk.NewLabel(configGB)
	if err != nil {
		return err
	}
	if err := configHint.SetText("After Start, add keys anytime — no restart needed. Hold any mapped key in each group to run that clicker. End toggles start/stop."); err != nil {
		return err
	}

	return timer.buildSection(page)
}

func (c *clickerTabController) buildSlot(parent walk.Container, index int) error {
	slotGB, err := walk.NewGroupBox(parent)
	if err != nil {
		return err
	}
	if err := slotGB.SetTitle(clickerSlotTitles[index]); err != nil {
		return err
	}
	rowLayout := walk.NewHBoxLayout()
	rowLayout.SetSpacing(10)
	if err := slotGB.SetLayout(rowLayout); err != nil {
		return err
	}

	w := &c.slots[index]

	keyText, err := walk.NewLabel(slotGB)
	if err != nil {
		return err
	}
	if err := keyText.SetText("Trigger keys:"); err != nil {
		return err
	}

	w.keyLabel, err = walk.NewLabel(slotGB)
	if err != nil {
		return err
	}
	if err := w.keyLabel.SetText(runner.KeysText(c.triggerVKs[index])); err != nil {
		return err
	}

	w.bindBtn, err = walk.NewPushButton(slotGB)
	if err != nil {
		return err
	}
	if err := w.bindBtn.SetText("Add key..."); err != nil {
		return err
	}
	slot := index
	w.bindBtn.Clicked().Attach(func() { c.bindKey(slot) })

	w.clearBtn, err = walk.NewPushButton(slotGB)
	if err != nil {
		return err
	}
	if err := w.clearBtn.SetText("Clear keys"); err != nil {
		return err
	}
	w.clearBtn.Clicked().Attach(func() { c.clearKeys(slot) })

	delayLabel, err := walk.NewLabel(slotGB)
	if err != nil {
		return err
	}
	if err := delayLabel.SetText("Delay (ms):"); err != nil {
		return err
	}

	w.delayEdit, err = walk.NewLineEdit(slotGB)
	if err != nil {
		return err
	}
	w.delayEdit.SetMaxLength(6)
	if err := w.delayEdit.SetMinMaxSize(walk.Size{Width: 80, Height: 0}, walk.Size{Width: 80, Height: 0}); err != nil {
		return err
	}
	if err := w.delayEdit.SetText(strconv.Itoa(runner.DefaultDelayMs)); err != nil {
		return err
	}
	c.lastLoggedDelay[index] = runner.DefaultDelayMs
	w.delayEdit.TextChanged().Attach(c.syncSettings)
	w.delayEdit.EditingFinished().Attach(func() { c.logDelayIfChanged(slot) })

	return nil
}

func (c *clickerTabController) config() runner.Config {
	cfg := runner.Config{Log: c.ctx.appendLog}
	for i := 0; i < runner.ClickerSlotCount; i++ {
		cfg.Slots[i] = runner.ClickerSlot{
			TriggerVKs: append([]int32(nil), c.triggerVKs[i]...),
			DelayMs:    c.delayMs(i),
			MouseClick: i == clickerWithMouse,
		}
	}
	return cfg
}

func (c *clickerTabController) delayMs(index int) int {
	if index < 0 || index >= runner.ClickerSlotCount {
		return runner.DefaultDelayMs
	}
	v, err := strconv.Atoi(c.slots[index].delayEdit.Text())
	if err != nil || v <= 0 {
		return runner.DefaultDelayMs
	}
	return v
}

func (c *clickerTabController) logDelayIfChanged(index int) {
	delay := c.delayMs(index)
	if delay == c.lastLoggedDelay[index] {
		return
	}
	c.lastLoggedDelay[index] = delay
	c.ctx.appendLog(fmt.Sprintf("%s delay: %d ms", clickerSlotTitles[index], delay))
}

func (c *clickerTabController) setEnabled(enabled bool) {
	for i := 0; i < runner.ClickerSlotCount; i++ {
		c.slots[i].delayEdit.SetEnabled(enabled)
		c.slots[i].bindBtn.SetEnabled(enabled)
		c.slots[i].clearBtn.SetEnabled(enabled)
	}
}

func (c *clickerTabController) updateKeyLabel(index int) {
	c.slots[index].keyLabel.SetText(runner.KeysText(c.triggerVKs[index]))
}

func (c *clickerTabController) clearKeys(index int) {
	if index < 0 || index >= runner.ClickerSlotCount {
		return
	}
	c.triggerVKs[index] = nil
	c.updateKeyLabel(index)
	c.ctx.appendLog(fmt.Sprintf("%s keys cleared", clickerSlotTitles[index]))
	c.syncSettings()
}

func (c *clickerTabController) bindKey(index int) {
	c.ctx.bindKeyFlow(
		func() bool {
			if !c.ctx.isViiperReady() || *c.ctx.bindActive || index < 0 || index >= runner.ClickerSlotCount {
				return false
			}
			*c.ctx.bindActive = true
			c.bindingSlot = index
			c.slots[index].bindBtn.SetEnabled(false)
			return true
		},
		fmt.Sprintf("Press a key to add for %s (%s timeout)...", clickerSlotTitles[index], runner.KeyBindTimeout),
		func() { c.bindingSlot = -1; *c.ctx.bindActive = false },
		func() { c.setEnabled(c.ctx.isViiperReady()) },
		func(vk int32) {
			c.ctx.unsetBinding(vk)
			c.triggerVKs[index] = append(c.triggerVKs[index], vk)
			c.updateKeyLabel(index)
			c.ctx.appendLog(fmt.Sprintf("%s added key %s", clickerSlotTitles[index], runner.KeyName(vk)))
			c.syncSettings()
		},
	)
}

func (c *clickerTabController) syncSettings() {
	cfg := c.config()
	c.ctx.mu.Lock()
	r := c.runner
	c.ctx.mu.Unlock()

	if r != nil && r.Running() {
		r.UpdateSettings(cfg.Slots)
	}
}

// unsetBinding removes vk from this controller's bindings. Returns true
// if the key was found and removed (so the caller can stop searching).
func (c *clickerTabController) unsetBinding(vk int32) bool {
	for i := 0; i < runner.ClickerSlotCount; i++ {
		for j, existing := range c.triggerVKs[i] {
			if existing == vk {
				c.triggerVKs[i] = append(c.triggerVKs[i][:j], c.triggerVKs[i][j+1:]...)
				c.updateKeyLabel(i)
				c.ctx.appendLog(fmt.Sprintf("Key %s removed from %s (reassigned)", runner.KeyName(vk), clickerSlotTitles[i]))
				c.syncSettings()
				return true
			}
		}
	}
	return false
}
