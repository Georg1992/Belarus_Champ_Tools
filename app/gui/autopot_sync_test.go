//go:build windows

package main

import (
	"testing"

	"belarus-champ-tools/runner"
)

type mockAutoPotRunner struct {
	running bool
	updates []runner.AutoPotConfig
}

func (m *mockAutoPotRunner) Running() bool { return m.running }

func (m *mockAutoPotRunner) UpdateSettings(cfg runner.AutoPotConfig) {
	m.updates = append(m.updates, cfg)
}

func TestCommittedThresholdFields(t *testing.T) {
	app := &guiApp{hpThreshold: 50, spThreshold: 30}
	if app.hpThreshold != 50 || app.spThreshold != 30 {
		t.Fatalf("thresholds=%d/%d want 50/30", app.hpThreshold, app.spThreshold)
	}
	app.hpThreshold = 60
	if app.hpThreshold != 60 || app.spThreshold != 30 {
		t.Fatalf("thresholds=%d/%d want 60/30", app.hpThreshold, app.spThreshold)
	}
}

func TestRunningRunnerReceivesThresholdUpdate(t *testing.T) {
	mock := &mockAutoPotRunner{running: true}
	cfg := runner.AutoPotConfig{
		HPEnabled:   true,
		HPKeyVK:     'W',
		HPThreshold: 60,
		SPThreshold: 50,
	}
	mock.UpdateSettings(cfg)
	if len(mock.updates) != 1 {
		t.Fatalf("updates=%d want 1", len(mock.updates))
	}
	if mock.updates[0].HPThreshold != 60 || mock.updates[0].SPThreshold != 50 {
		t.Fatalf("update=%+v want 60/50", mock.updates[0])
	}
}
