// Package runner is the public facade. It re-exports a small set of
// types from runner/autopot, runner/internal/lifecycle, and
// runner/internal/session (see also timing.go for timing constants)
// so consumers (gui, tests) can import a single package instead of
// reaching into the internal subpackages.
//
// The facade is intentionally lean — only symbols with a current
// consumer are re-exported. Add a new re-export inside the type/var
// blocks below when something needs it; remove it when nothing uses it.
//
// Note: this is the ONLY place in the package that imports runner/autopot
// (autopot does not import runner to keep the graph cycle-free). All
// other files in runner/* import from internal/{lifecycle,session,
// timing} directly.
package runner

import (
	"belarus-champ-tools/runner/autopot"
	"belarus-champ-tools/runner/internal/lifecycle"
	"belarus-champ-tools/runner/internal/session"
	win "belarus-champ-tools/runner/platform/windows"

	"golang.org/x/sys/windows"
)

// InputSession is the canonical input-device interface any runner uses.
// Re-exported so gui/* and tests can keep writing runner.InputSession.
type InputSession = session.InputSession

// Lifecycle is the generic goroutine-lifecycle helper.
type Lifecycle[C any] = lifecycle.Lifecycle[C]

// AutoPot types re-exported for callers that import only this package.
type (
	AutoPotRunner = autopot.AutoPotRunner
	AutoPotConfig = autopot.AutoPotConfig
)

var (
	NewAutoPot = autopot.NewAutoPot
)

// OpenGameProcess opens a handle to the process with the given PID
// for memory reading (PROCESS_VM_READ | PROCESS_QUERY_INFORMATION).
// Returns 0 and an error on failure. The returned handle must be
// closed with CloseGameProcess when no longer needed.
func OpenGameProcess(pid uint32) (uintptr, error) {
	h, err := win.OpenProcessHandle(pid)
	if err != nil {
		return 0, err
	}
	return uintptr(h), nil
}

// CloseGameProcess closes a process handle obtained from
// OpenGameProcess. Safe to call with 0.
func CloseGameProcess(h uintptr) {
	if h != 0 {
		win.CloseProcessHandle(windows.Handle(h))
	}
}

// ProcessInfo holds process details for the GUI process selector.
type ProcessInfo struct {
	PID  uint32
	Name string
}

// ListGameProcesses returns all running processes sorted by name,
// suitable for populating a process combo box.
func ListGameProcesses() ([]ProcessInfo, error) {
	procs, err := win.ListProcesses()
	if err != nil {
		return nil, err
	}
	out := make([]ProcessInfo, len(procs))
	for i, p := range procs {
		out[i] = ProcessInfo{PID: p.PID, Name: p.Name}
	}
	return out, nil
}

