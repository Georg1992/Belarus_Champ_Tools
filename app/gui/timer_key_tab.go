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

func (a *guiApp) buildTimerKeySection(page *walk.TabPage) error {
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

	a.clicker.timerVisibleCount = 1
	for i := 0; i < runner.TimerKeySlotCount; i++ {
		if err := a.buildTimerSlotRow(slotsContainer, i); err != nil {
			return err
		}
		if i > 0 {
			a.clicker.timerSlots[i].row.SetVisible(false)
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

	a.clicker.timerAddBtn, err = walk.NewPushButton(addRow)
	if err != nil {
		return err
	}
	if err := a.clicker.timerAddBtn.SetText("+ Add timer"); err != nil {
		return err
	}
	a.clicker.timerAddBtn.Clicked().Attach(a.onAddTimer)

	timerHint, err := walk.NewLabel(timerGB)
	if err != nil {
		return err
	}
	if err := timerHint.SetText("Each enabled timer presses its key once every interval. Keyboard only — separate from the clicker above."); err != nil {
		return err
	}

	return nil
}

func (a *guiApp) buildTimerSlotRow(parent walk.Container, index int) error {
	row, err := walk.NewComposite(parent)
	if err != nil {
		return err
	}
	rowLayout := walk.NewHBoxLayout()
	rowLayout.SetSpacing(10)
	if err := row.SetLayout(rowLayout); err != nil {
		return err
	}

	w := &a.clicker.timerSlots[index]
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
	w.enabledCB.CheckedChanged().Attach(a.syncTimerKeySettings)

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
	w.bindBtn.Clicked().Attach(func() {
		a.bindTimerKey(slot)
	})

	w.clearBtn, err = walk.NewPushButton(row)
	if err != nil {
		return err
	}
	if err := w.clearBtn.SetText("Clear"); err != nil {
		return err
	}
	w.clearBtn.Clicked().Attach(func() {
		a.clearTimerKey(slot)
	})

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
	w.intervalEdit.TextChanged().Attach(a.syncTimerKeySettings)

	return nil
}

func (a *guiApp) onAddTimer() {
	if a.clicker.timerVisibleCount >= runner.TimerKeySlotCount {
		return
	}
	a.clicker.timerSlots[a.clicker.timerVisibleCount].row.SetVisible(true)
	a.clicker.timerVisibleCount++
	a.updateTimerAddButton()
}

func (a *guiApp) updateTimerAddButton() {
	if a.clicker.timerAddBtn == nil {
		return
	}
	atMax := a.clicker.timerVisibleCount >= runner.TimerKeySlotCount
	a.clicker.timerAddBtn.SetVisible(!atMax)
}

func (a *guiApp) timerKeyConfig() runner.TimerKeyConfig {
	cfg := runner.TimerKeyConfig{
		Log: a.appendLog,
	}
	for i := 0; i < a.clicker.timerVisibleCount; i++ {
		cfg.Slots[i] = runner.TimerSlot{
			Enabled:    a.clicker.timerSlots[i].enabledCB.Checked(),
			KeyVK:      a.clicker.timerKeyVKs[i],
			IntervalMs: a.timerIntervalMs(i),
		}
	}
	return cfg
}

func (a *guiApp) timerKeyWanted() runner.TimerKeyConfig {
	cfg := a.timerKeyConfig()
	for i := 0; i < a.clicker.timerVisibleCount; i++ {
		if !cfg.Slots[i].Enabled || cfg.Slots[i].KeyVK == 0 {
			cfg.Slots[i].Enabled = false
		}
	}
	return cfg
}

func (a *guiApp) timerIntervalMs(index int) int {
	if index < 0 || index >= a.clicker.timerVisibleCount {
		return runner.DefaultTimerKeyIntervalMs
	}
	v, err := strconv.Atoi(a.clicker.timerSlots[index].intervalEdit.Text())
	if err != nil || v <= 0 {
		return runner.DefaultTimerKeyIntervalMs
	}
	return v * 1000
}

func (a *guiApp) syncTimerKeySettings() {
	cfg := a.timerKeyWanted()
	a.mu.Lock()
	t := a.timerKeyRunner
	a.mu.Unlock()

	if t != nil && t.Running() {
		if !cfg.AnyActive() {
			// Nil the runner immediately so isStarted() and
			// subsequent sync calls see a stopped state.
			a.mu.Lock()
			a.timerKeyRunner = nil
			a.mu.Unlock()
			// Stop+Wait on a background goroutine to avoid
			// deadlocking the GUI thread if the runner
			// goroutine is in a Synchronize call.
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

	if a.isStarted() {
		a.startTimerKeyRunner(cfg, a.guiLog(a.appendLog))
	}
}

func (a *guiApp) setTimerKeyConfigEnabled(enabled bool) {
	for i := 0; i < a.clicker.timerVisibleCount; i++ {
		a.clicker.timerSlots[i].enabledCB.SetEnabled(enabled)
		a.clicker.timerSlots[i].intervalEdit.SetEnabled(enabled)
		a.clicker.timerSlots[i].bindBtn.SetEnabled(enabled)
		a.clicker.timerSlots[i].clearBtn.SetEnabled(enabled)
	}
	if a.clicker.timerAddBtn != nil {
		a.clicker.timerAddBtn.SetEnabled(enabled && a.clicker.timerVisibleCount < runner.TimerKeySlotCount)
	}
}

func (a *guiApp) startTimerKeyRunner(cfg runner.TimerKeyConfig, log func(string)) {
	take, store := makeLifecycleSlot[*runner.TimerKeyRunner](&a.mu, &a.timerKeyRunner)
	startLifecycle(
		take, store,
		"Timer keys",
		log,
		func() runner.InputSession {
			a.mu.Lock()
			defer a.mu.Unlock()
			return a.inputSession
		},
		func() bool { return cfg.AnyActive() },
		func(sess runner.InputSession) *runner.TimerKeyRunner {
			cfg.Session = sess
			cfg.Log = log
			return runner.NewTimerKey(cfg)
		},
	)
}

func (a *guiApp) clearTimerKey(index int) {
	if index < 0 || index >= a.clicker.timerVisibleCount {
		return
	}
	a.clicker.timerKeyVKs[index] = 0
	a.clicker.timerSlots[index].keyLabel.SetText("none")
	a.appendLog(fmt.Sprintf("Timer %d key cleared", index+1))
	a.syncTimerKeySettings()
}

func (a *guiApp) bindTimerKey(index int) {
	a.bindKeyFlow(
		func() bool {
			if !a.isViiperReady() || a.bindingActive || index < 0 || index >= a.clicker.timerVisibleCount {
				return false
			}
			a.bindingActive = true
			a.clicker.timerSlots[index].bindBtn.SetEnabled(false)
			return true
		},
		fmt.Sprintf("Press a key for timer %d (%s timeout)...", index+1, runner.KeyBindTimeout),
		func() { a.bindingActive = false },
		func() { a.setTimerKeyConfigEnabled(a.isViiperReady()) },
		func(vk int32) {
			a.unsetKeyBinding(vk)
			a.clicker.timerKeyVKs[index] = vk
			a.clicker.timerSlots[index].keyLabel.SetText(runner.KeyName(vk))
			a.appendLog(fmt.Sprintf("Timer %d key: %s", index+1, runner.KeyName(vk)))
			a.syncTimerKeySettings()
		},
	)
}
