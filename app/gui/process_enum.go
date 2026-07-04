//go:build windows

package main

import (
	"sort"

	"belarus-champ-tools/runner"

	"github.com/lxn/walk"
)

// processInfo holds PID and executable name for combo box selection.
type processInfo struct {
	PID  uint32
	Name string
}

// listProcesses returns all running processes with non-empty names,
// sorted alphabetically by name.
func listProcesses() ([]processInfo, error) {
	procs, err := runner.ListGameProcesses()
	if err != nil {
		return nil, err
	}
	result := make([]processInfo, 0, len(procs))
	for _, p := range procs {
		result = append(result, processInfo{PID: p.PID, Name: p.Name})
	}
	// Already sorted by ListProcesses, but ensure stable order.
	sort.Slice(result, func(i, j int) bool {
		return result[i].Name < result[j].Name
	})
	return result, nil
}

// populateProcessComboBox fills a walk ComboBox with running process names.
// Returns the full process info list so the caller can map selection to PID.
func populateProcessComboBox(cb *walk.ComboBox) ([]processInfo, error) {
	items, err := listProcesses()
	if err != nil {
		return nil, err
	}

	// Save current selection to restore if still visible.
	selIdx := cb.CurrentIndex()
	selName := ""
	if selIdx >= 0 {
		selName = cb.Text()
	}

	// Clear and repopulate.
	cb.SetModel(nil)
	names := make([]string, 0, len(items))
	for _, p := range items {
		names = append(names, p.Name)
	}
	if err := cb.SetModel(names); err != nil {
		return nil, err
	}

	// Restore selection if the same process name still exists.
	if selName != "" {
		for i, n := range names {
			if n == selName {
				cb.SetCurrentIndex(i)
				break
			}
		}
	}

	return items, nil
}
