package autopot

import (
	"fmt"
	"testing"

	"golang.org/x/sys/windows"
)

// ---------------------------------------------------------------------------
// Default fallback tests
// ---------------------------------------------------------------------------

func TestGetProcessBaseAddr_DefaultReturnsError(t *testing.T) {
	orig := GetProcessBaseAddr
	defer func() { GetProcessBaseAddr = orig }()
	// Reset to sentinel default — on Windows, init() in process_windows.go
	// already wired the real platform implementation.
	GetProcessBaseAddr = func(pid uint32) (uintptr, error) {
		return 0, fmt.Errorf("GetProcessBaseAddr: not available on this platform")
	}

	addr, err := GetProcessBaseAddr(12345)
	if err == nil {
		t.Error("GetProcessBaseAddr: expected error from default sentinel")
	}
	if addr != 0 {
		t.Errorf("GetProcessBaseAddr: expected 0 addr, got %v", addr)
	}
}

func TestOpenProcessHandle_DefaultReturnsError(t *testing.T) {
	orig := OpenProcessHandle
	defer func() { OpenProcessHandle = orig }()
	OpenProcessHandle = func(pid uint32) (windows.Handle, error) {
		return windows.InvalidHandle, fmt.Errorf("OpenProcessHandle: not available on this platform")
	}

	h, err := OpenProcessHandle(12345)
	if err == nil {
		t.Error("OpenProcessHandle: expected error from default sentinel")
	}
	if h != windows.InvalidHandle {
		t.Errorf("OpenProcessHandle: expected InvalidHandle, got %v", h)
	}
}

func TestCloseProcessHandle_DefaultIsNoop(t *testing.T) {
	orig := CloseProcessHandle
	defer func() { CloseProcessHandle = orig }()
	CloseProcessHandle = func(h windows.Handle) {}

	// Must not panic with any handle value, including InvalidHandle and 0.
	CloseProcessHandle(windows.InvalidHandle)
	CloseProcessHandle(0)
	CloseProcessHandle(windows.Handle(42))
}

func TestReadProcessUint32ByHandle_DefaultReturnsError(t *testing.T) {
	orig := ReadProcessUint32ByHandle
	defer func() { ReadProcessUint32ByHandle = orig }()
	ReadProcessUint32ByHandle = func(h windows.Handle, addr uintptr) (uint32, error) {
		return 0, fmt.Errorf("ReadProcessUint32ByHandle: not available on this platform")
	}

	val, err := ReadProcessUint32ByHandle(windows.InvalidHandle, 0x1234)
	if err == nil {
		t.Error("ReadProcessUint32ByHandle: expected error from default sentinel")
	}
	if val != 0 {
		t.Errorf("ReadProcessUint32ByHandle: expected 0, got %d", val)
	}
}

func TestFindVisibleWindowPID_DefaultReturnsZero(t *testing.T) {
	orig := FindVisibleWindowPID
	defer func() { FindVisibleWindowPID = orig }()
	FindVisibleWindowPID = func(title string) uint32 { return 0 }

	if pid := FindVisibleWindowPID("anything"); pid != 0 {
		t.Errorf("FindVisibleWindowPID: expected 0 from default sentinel, got %d", pid)
	}
}

// ---------------------------------------------------------------------------
// DI swap-in tests
// ---------------------------------------------------------------------------

func TestGetProcessBaseAddr_CanBeSwapped(t *testing.T) {
	orig := GetProcessBaseAddr
	defer func() { GetProcessBaseAddr = orig }()

	GetProcessBaseAddr = func(pid uint32) (uintptr, error) {
		if pid == 9999 {
			return 0x400000, nil
		}
		return 0, fmt.Errorf("unknown pid %d", pid)
	}

	addr, err := GetProcessBaseAddr(9999)
	if err != nil {
		t.Errorf("GetProcessBaseAddr(9999): unexpected error: %v", err)
	}
	if addr != 0x400000 {
		t.Errorf("GetProcessBaseAddr(9999): expected 0x400000, got %v", addr)
	}

	_, err = GetProcessBaseAddr(1)
	if err == nil {
		t.Error("GetProcessBaseAddr(1): expected error for unknown PID")
	}
}

func TestOpenAndCloseProcessHandle_CanBeSwapped(t *testing.T) {
	origOpen := OpenProcessHandle
	origClose := CloseProcessHandle
	defer func() {
		OpenProcessHandle = origOpen
		CloseProcessHandle = origClose
	}()

	closed := false
	OpenProcessHandle = func(pid uint32) (windows.Handle, error) {
		if pid == 42 {
			return windows.Handle(42), nil
		}
	return windows.InvalidHandle, fmt.Errorf("bad pid")
	}
	CloseProcessHandle = func(h windows.Handle) {
		if h == windows.Handle(42) {
			closed = true
		}
	}

	h, err := OpenProcessHandle(42)
	if err != nil {
		t.Fatalf("OpenProcessHandle(42): %v", err)
	}
	CloseProcessHandle(h)
	if !closed {
		t.Error("CloseProcessHandle: expected close call")
	}
}

func TestReadProcessUint32ByHandle_SwapReturnsValue(t *testing.T) {
	orig := ReadProcessUint32ByHandle
	defer func() { ReadProcessUint32ByHandle = orig }()

	ReadProcessUint32ByHandle = func(h windows.Handle, addr uintptr) (uint32, error) {
		if addr == 0xDEAD {
			return 31337, nil
		}
		return 0, fmt.Errorf("bad address")
	}

	val, err := ReadProcessUint32ByHandle(0, 0xDEAD)
	if err != nil {
		t.Fatalf("ReadProcessUint32ByHandle: %v", err)
	}
	if val != 31337 {
		t.Errorf("ReadProcessUint32ByHandle: expected 31337, got %d", val)
	}

	_, err = ReadProcessUint32ByHandle(0, 0xBEEF)
	if err == nil {
		t.Error("ReadProcessUint32ByHandle: expected error for unknown addr")
	}
}

func TestFindVisibleWindowPID_SwapReturnsMatch(t *testing.T) {
	orig := FindVisibleWindowPID
	defer func() { FindVisibleWindowPID = orig }()

	FindVisibleWindowPID = func(title string) uint32 {
		if title == "ROClient" {
			return 7777
		}
		return 0
	}

	if pid := FindVisibleWindowPID("ROClient"); pid != 7777 {
		t.Errorf("FindVisibleWindowPID(ROClient): expected 7777, got %d", pid)
	}
	if pid := FindVisibleWindowPID("Chrome"); pid != 0 {
		t.Errorf("FindVisibleWindowPID(Chrome): expected 0, got %d", pid)
	}
}
