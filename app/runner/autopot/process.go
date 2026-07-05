package autopot

import (
	"fmt"

	"golang.org/x/sys/windows"
)

// GetProcessBaseAddr returns the base address of the first module in the
// process with the given PID. Defaults to a sentinel error; the real
// implementation is wired via init() in process_windows.go.
var GetProcessBaseAddr = func(pid uint32) (uintptr, error) {
	return 0, fmt.Errorf("GetProcessBaseAddr: not available on this platform")
}

// OpenProcessHandle opens a handle to the process with the given PID
// for memory reading. Defaults to an error; the real implementation is
// wired via init() in process_windows.go.
var OpenProcessHandle = func(pid uint32) (windows.Handle, error) {
	return windows.InvalidHandle, fmt.Errorf("OpenProcessHandle: not available on this platform")
}

// CloseProcessHandle closes a process handle. Defaults to a no-op.
var CloseProcessHandle = func(h windows.Handle) {}

// ReadProcessUint32ByHandle reads a 32-bit value from the target process
// using an open handle. Defaults to an error.
var ReadProcessUint32ByHandle = func(h windows.Handle, addr uintptr) (uint32, error) {
	return 0, fmt.Errorf("ReadProcessUint32ByHandle: not available on this platform")
}

// FindVisibleWindowPID searches visible windows for one whose title
// contains the given substring and returns its PID. Defaults to 0.
var FindVisibleWindowPID = func(title string) uint32 {
	return 0
}
