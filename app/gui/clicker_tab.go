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

// clickerController owns clicker + timer key state and config building.
type clickerController struct {
	slots              [runner.ClickerSlotCount]clickerSlotWidgets
	triggerVKs         [runner.ClickerSlotCount][]int32
	lastLoggedDelay    [runner.ClickerSlotCount]int

	timerSlots        [runner.TimerKeySlotCount]timerSlotWidgets
	timerKeyVKs       [runner.TimerKeySlotCount]int32
	timerVisibleCount int
	timerAddBtn       *walk.PushButton
}

func (c *clickerController) config(logFn func(string)) runner.Config {
	cfg := runner.Config{Log: logFn}
	for i := 0; i < runner.ClickerSlotCount; i++ {
		cfg.Slots[i] = runner.ClickerSlot{
			TriggerVKs: append([]int32(nil), c.triggerVKs[i]...),
			DelayMs:    c.delayMs(i),
			MouseClick: i == clickerWithMouse,
		}
	}
	return cfg
}

func (c *clickerController) delayMs(index int) int {
	if index < 0 || index >= runner.ClickerSlotCount {
		return runner.DefaultDelayMs
	}
	v, err := strconv.Atoi(c.slots[index].delayEdit.Text())
	if err != nil || v <= 0 {
		return runner.DefaultDelayMs
	}
	return v
}

func (c *clickerController) timerConfig(logFn func(string)) runner.TimerKeyConfig {
	cfg := runner.TimerKeyConfig{Log: logFn}
	for i := 0; i < c.timerVisibleCount; i++ {
		cfg.Slots[i] = runner.TimerSlot{
			Enabled:    c.timerSlots[i].enabledCB.Checked(),
			KeyVK:      c.timerKeyVKs[i],
			IntervalMs: c.timerIntervalMs(i),
		}
	}
	return cfg
}

func (c *clickerController) timerWanted(logFn func(string)) runner.TimerKeyConfig {
	cfg := c.timerConfig(logFn)
	for i := 0; i < c.timerVisibleCount; i++ {
		if !cfg.Slots[i].Enabled || cfg.Slots[i].KeyVK == 0 {
			cfg.Slots[i].Enabled = false
		}
	}
	return cfg
}

func (c *clickerController) timerIntervalMs(index int) int {
	if index < 0 || index >= c.timerVisibleCount {
		return runner.DefaultTimerKeyIntervalMs
	}
	v, err := strconv.Atoi(c.timerSlots[index].intervalEdit.Text())
	if err != nil || v <= 0 {
		return runner.DefaultTimerKeyIntervalMs
	}
	return v * 1000
}

func (a *guiApp) buildClickerTab(page *walk.TabPage) error {
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
		if err := a.buildClickerSlot(configGB, i); err != nil {
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

	return a.buildTimerKeySection(page)
}

func (a *guiApp) buildClickerSlot(parent walk.Container, index int) error {
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

	w := &a.clicker.slots[index]

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
	if err := w.keyLabel.SetText(runner.KeysText(a.clicker.triggerVKs[index])); err != nil {
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
	w.bindBtn.Clicked().Attach(func() {
		a.bindClickerKey(slot)
	})

	w.clearBtn, err = walk.NewPushButton(slotGB)
	if err != nil {
		return err
	}
	if err := w.clearBtn.SetText("Clear keys"); err != nil {
		return err
	}
	w.clearBtn.Clicked().Attach(func() {
		a.clearClickerKey(slot)
	})

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
	a.clicker.lastLoggedDelay[index] = runner.DefaultDelayMs
	w.delayEdit.TextChanged().Attach(a.syncRunnerSettings)
	w.delayEdit.EditingFinished().Attach(func() {
		a.logClickerDelayIfChanged(slot)
	})

	return nil
}

func (a *guiApp) clickerDelayMs(index int) int {
	if index < 0 || index >= runner.ClickerSlotCount {
		return runner.DefaultDelayMs
	}
	v, err := strconv.Atoi(a.clicker.slots[index].delayEdit.Text())
	if err != nil || v <= 0 {
		return runner.DefaultDelayMs
	}
	return v
}

func (a *guiApp) logClickerDelayIfChanged(index int) {
	delay := a.clickerDelayMs(index)
	if delay == a.clicker.lastLoggedDelay[index] {
		return
	}
	a.clicker.lastLoggedDelay[index] = delay
	a.appendLog(fmt.Sprintf("%s delay: %d ms", clickerSlotTitles[index], delay))
}

func (a *guiApp) setClickerConfigEnabled(enabled bool) {
	for i := 0; i < runner.ClickerSlotCount; i++ {
		a.clicker.slots[i].delayEdit.SetEnabled(enabled)
		a.clicker.slots[i].bindBtn.SetEnabled(enabled)
		a.clicker.slots[i].clearBtn.SetEnabled(enabled)
	}
}

func (a *guiApp) updateClickerKeyLabel(index int) {
	a.clicker.slots[index].keyLabel.SetText(runner.KeysText(a.clicker.triggerVKs[index]))
}

func (a *guiApp) clearClickerKey(index int) {
	if index < 0 || index >= runner.ClickerSlotCount {
		return
	}
	a.clicker.triggerVKs[index] = nil
	a.updateClickerKeyLabel(index)
	a.appendLog(fmt.Sprintf("%s keys cleared", clickerSlotTitles[index]))
	a.syncRunnerSettings()
}

func (a *guiApp) bindClickerKey(index int) {
	a.bindKeyFlow(
		func() bool {
			if !a.isViiperReady() || a.bindingActive || index < 0 || index >= runner.ClickerSlotCount {
				return false
			}
			a.bindingActive = true
			a.clicker.slots[index].bindBtn.SetEnabled(false)
			return true
		},
		fmt.Sprintf("Press a key to add for %s (%s timeout)...", clickerSlotTitles[index], runner.KeyBindTimeout),
		func() { a.bindingActive = false },
		func() { a.setClickerConfigEnabled(a.isViiperReady()) },
		func(vk int32) {
			a.unsetKeyBinding(vk)
			a.clicker.triggerVKs[index] = append(a.clicker.triggerVKs[index], vk)
			a.updateClickerKeyLabel(index)
			a.appendLog(fmt.Sprintf("%s added key %s", clickerSlotTitles[index], runner.KeyName(vk)))
			a.syncRunnerSettings()
		},
	)
}
