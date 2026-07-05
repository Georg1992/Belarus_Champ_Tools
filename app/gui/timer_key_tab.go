//go:build windows

package main

import (
	"fmt"
	"os"
	"runtime/debug"
	"strconv"

	"belarus-champ-tools/runner"
	"github.com/lxn/walk"
)

type timerSlotWidgets struct {
	row          *walk.Composite
	enabledCB    *walk.CheckBox
	keyLabel     *walk.Label
	bindBtn      *walk.PushButton
	clearBtn     *walk.PushButton
	intervalEdit *walk.LineEdit
}

// timerKeyTabController manages the Timer Key section and its runner.
type timerKeyTabController struct {
	ctx *tabContext

	slots        [runner.TimerKeySlotCount]timerSlotWidgets
	keyVKs       [runner.TimerKeySlotCount]int32
	visibleCount int
	addBtn       *walk.PushButton
	bindingSlot  int

	runner *runner.TimerKeyRunner
}

func newTimerKeyTabController(ctx *tabContext) *timerKeyTabController {
	return &timerKeyTabController{
		ctx:          ctx,
		bindingSlot:  -1,
		visibleCount: 1,
	}
}

func (c *timerKeyTabController) runnerPtr() **runner.TimerKeyRunner { return &c.runner }

// buildSection builds the timer key UI inside the clicker tab page.
func (c *timerKeyTabController) buildSection(page *walk.TabPage) error {
	timerGB, err := walk.NewGroupBox(page)
	if err != nil {
		return err
	}
	if err := timerGB.SetTitle("3. Timer keys"); err != nil {
		return err
	}
	timerLayout := walk.NewVBoxLayout()
	timerLayout.SetSpacing(8)
	if err := timerGB.SetLayout(timerLayout); err != nil {
		return err
	}

	slotsContainer, err := walk.NewComposite(timerGB)
	if err != nil {
		return err
	}
	slotsLayout := walk.NewVBoxLayout()
	slotsLayout.SetSpacing(6)
	if err := slotsContainer.SetLayout(slotsLayout); err != nil {
		return err
	}

	for i := 0; i < runner.TimerKeySlotCount; i++ {
		if err := c.buildSlotRow(slotsContainer, i); err != nil {
			return err
		}
		if i > 0 {
			c.slots[i].row.SetVisible(false)
		}
	}

	addRow, err := walk.NewComposite(timerGB)
	if err != nil {
		return err
	}
	addLayout := walk.NewHBoxLayout()
	addLayout.SetSpacing(10)
	if err := addRow.SetLayout(addLayout); err != nil {
		return err
	}

	c.addBtn, err = walk.NewPushButton(addRow)
	if err != nil {
		return err
	}
	if err := c.addBtn.SetText("+ Add timer"); err != nil {
		return err
	}
	c.addBtn.Clicked().Attach(c.onAdd)

	timerHint, err := walk.NewLabel(timerGB)
	if err != nil {
		return err
	}
	if err := timerHint.SetText("Each enabled timer presses its key once every interval. Keyboard only — separate from the clicker above."); err != nil {
		return err
	}

	return nil
}

func (c *timerKeyTabController) buildSlotRow(parent walk.Container, index int) error {
	row, err := walk.NewComposite(parent)
	if err != nil {
		return err
	}
	rowLayout := walk.NewHBoxLayout()
	rowLayout.SetSpacing(10)
	if err := row.SetLayout(rowLayout); err != nil {
		return err
	}

	w := &c.slots[index]
	w.row = row

	slotLabel, err := walk.NewLabel(row)
	if err != nil {
		return err
	}
	if err := slotLabel.SetText(fmt.Sprintf("Timer %d:", index+1)); err != nil {
		return err
	}

	w.enabledCB, err = walk.NewCheckBox(row)
	if err != nil {
		return err
	}
	w.enabledCB.CheckedChanged().Attach(c.syncSettings)

	keyText, err := walk.NewLabel(row)
	if err != nil {
		return err
	}
	if err := keyText.SetText("Key:"); err != nil {
		return err
	}

	w.keyLabel, err = walk.NewLabel(row)
	if err != nil {
		return err
	}
	if err := w.keyLabel.SetText("none"); err != nil {
		return err
	}

	w.bindBtn, err = walk.NewPushButton(row)
	if err != nil {
		return err
	}
	if err := w.bindBtn.SetText("Set key..."); err != nil {
		return err
	}
	slot := index
	w.bindBtn.Clicked().Attach(func() { c.bindKey(slot) })

	w.clearBtn, err = walk.NewPushButton(row)
	if err != nil {
		return err
	}
	if err := w.clearBtn.SetText("Clear"); err != nil {
		return err
	}
	w.clearBtn.Clicked().Attach(func() { c.clearKey(slot) })

	intervalLabel, err := walk.NewLabel(row)
	if err != nil {
		return err
	}
	if err := intervalLabel.SetText("Interval (s):"); err != nil {
		return err
	}

	w.intervalEdit, err = walk.NewLineEdit(row)
	if err != nil {
		return err
	}
	w.intervalEdit.SetMaxLength(6)
	if err := w.intervalEdit.SetMinMaxSize(walk.Size{Width: 80, Height: 0}, walk.Size{Width: 80, Height: 0}); err != nil {
		return err
	}
	if err := w.intervalEdit.SetText(strconv.Itoa(runner.DefaultTimerKeyIntervalSec)); err != nil {
		return err
	}
	w.intervalEdit.TextChanged().Attach(c.syncSettings)

	return nil
}

func (c *timerKeyTabController) onAdd() {
	if c.visibleCount >= runner.TimerKeySlotCount {
		return
	}
	c.slots[c.visibleCount].row.SetVisible(true)
	c.visibleCount++
	c.updateAddButton()
}

func (c *timerKeyTabController) updateAddButton() {
	if c.addBtn == nil {
		return
	}
	atMax := c.visibleCount >= runner.TimerKeySlotCount
	c.addBtn.SetVisible(!atMax)
}

func (c *timerKeyTabController) rawConfig() runner.TimerKeyConfig {
	cfg := runner.TimerKeyConfig{Log: c.ctx.appendLog}
	for i := 0; i < c.visibleCount; i++ {
		cfg.Slots[i] = runner.TimerSlot{
			Enabled:    c.slots[i].enabledCB.Checked(),
			KeyVK:      c.keyVKs[i],
			IntervalMs: c.intervalMs(i),
		}
	}
	return cfg
}

func (c *timerKeyTabController) wanted() runner.TimerKeyConfig {
	cfg := c.rawConfig()
	for i := 0; i < c.visibleCount; i++ {
		if !cfg.Slots[i].Enabled || cfg.Slots[i].KeyVK == 0 {
			cfg.Slots[i].Enabled = false
		}
	}
	return cfg
}

func (c *timerKeyTabController) intervalMs(index int) int {
	if index < 0 || index >= c.visibleCount {
		return runner.DefaultTimerKeyIntervalMs
	}
	v, err := strconv.Atoi(c.slots[index].intervalEdit.Text())
	if err != nil || v <= 0 {
		return runner.DefaultTimerKeyIntervalMs
	}
	return v * 1000
}

func (c *timerKeyTabController) syncSettings() {
	cfg := c.wanted()
	c.ctx.mu.Lock()
	t := c.runner
	c.ctx.mu.Unlock()

	if t != nil && t.Running() {
		if !cfg.AnyActive() {
			c.ctx.mu.Lock()
			c.runner = nil
			c.ctx.mu.Unlock()
			go func(old *runner.TimerKeyRunner) {
				defer func() {
					if r := recover(); r != nil {
						_, _ = fmt.Fprintf(os.Stderr, "PANIC in timerKey stop: %v\n%s\n", r, debug.Stack())
					}
				}()
				old.Stop()
				old.Wait()
			}(t)
			return
		}
		t.UpdateSettings(cfg)
		return
	}

	if c.ctx.isStarted() {
		c.startRunner(cfg, c.ctx.guiLog(c.ctx.appendLog))
	}
}

func (c *timerKeyTabController) setEnabled(enabled bool) {
	for i := 0; i < c.visibleCount; i++ {
		c.slots[i].enabledCB.SetEnabled(enabled)
		c.slots[i].intervalEdit.SetEnabled(enabled)
		c.slots[i].bindBtn.SetEnabled(enabled)
		c.slots[i].clearBtn.SetEnabled(enabled)
	}
	if c.addBtn != nil {
		c.addBtn.SetEnabled(enabled && c.visibleCount < runner.TimerKeySlotCount)
	}
}

func (c *timerKeyTabController) startRunner(cfg runner.TimerKeyConfig, log func(string)) {
	take, store := makeLifecycleSlot[*runner.TimerKeyRunner](c.ctx.mu, c.runnerPtr())
	startLifecycle(
		take, store,
		"Timer keys", log,
		func() runner.InputSession { return c.ctx.session() },
		func() bool { return cfg.AnyActive() },
		func(sess runner.InputSession) *runner.TimerKeyRunner {
			cfg.Session = sess
			cfg.Log = log
			return runner.NewTimerKey(cfg)
		},
	)
}

func (c *timerKeyTabController) clearKey(index int) {
	if index < 0 || index >= c.visibleCount {
		return
	}
	c.keyVKs[index] = 0
	c.slots[index].keyLabel.SetText("none")
	c.ctx.appendLog(fmt.Sprintf("Timer %d key cleared", index+1))
	c.syncSettings()
}

func (c *timerKeyTabController) bindKey(index int) {
	c.ctx.bindKeyFlow(
		func() bool {
			if !c.ctx.isViiperReady() || *c.ctx.bindActive || index < 0 || index >= c.visibleCount {
				return false
			}
			*c.ctx.bindActive = true
			c.bindingSlot = index
			c.slots[index].bindBtn.SetEnabled(false)
			return true
		},
		fmt.Sprintf("Press a key for timer %d (%s timeout)...", index+1, runner.KeyBindTimeout),
		func() { c.bindingSlot = -1; *c.ctx.bindActive = false },
		func() { c.setEnabled(c.ctx.isViiperReady()) },
		func(vk int32) {
			c.ctx.unsetBinding(vk)
			c.keyVKs[index] = vk
			c.slots[index].keyLabel.SetText(runner.KeyName(vk))
			c.ctx.appendLog(fmt.Sprintf("Timer %d key: %s", index+1, runner.KeyName(vk)))
			c.syncSettings()
		},
	)
}

// unsetBinding removes vk from this controller. Returns true if removed.
func (c *timerKeyTabController) unsetBinding(vk int32) bool {
	for i := 0; i < c.visibleCount; i++ {
		if c.keyVKs[i] == vk {
			c.keyVKs[i] = 0
			c.slots[i].keyLabel.SetText("none")
			c.ctx.appendLog(fmt.Sprintf("Key %s removed from Timer %d (reassigned)", runner.KeyName(vk), i+1))
			c.syncSettings()
			return true
		}
	}
	return false
}
