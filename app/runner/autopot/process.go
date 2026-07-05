package autopot

import "fmt"

// GetProcessBaseAddr returns the base address of the first module in the
// process with the given PID. Defaults to a sentinel error; the real
// implementation is wired via init() in process_getter_windows.go.
var GetProcessBaseAddr = func(pid uint32) (uintptr, error) {
	return 0, fmt.Errorf("GetProcessBaseAddr: not available on this platform")
}
