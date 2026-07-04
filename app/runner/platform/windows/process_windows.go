//go:build windows

package runner

import (
	"fmt"
	"sort"
	"unsafe"

	"golang.org/x/sys/windows"
)

// ProcessInfo holds details about a running process for memory reading.
type ProcessInfo struct {
	PID  uint32
	Name string // executable name, e.g. "Ragnarok.exe"
}

var (
	procKernel32               = windows.NewLazySystemDLL("kernel32.dll")
	procCreateToolhelp32Snapshot = procKernel32.NewProc("CreateToolhelp32Snapshot")
	procProcess32First         = procKernel32.NewProc("Process32FirstW")
	procProcess32Next          = procKernel32.NewProc("Process32NextW")
	procOpenProcess            = procKernel32.NewProc("OpenProcess")
	procReadProcessMemory      = procKernel32.NewProc("ReadProcessMemory")
	procCloseHandle            = procKernel32.NewProc("CloseHandle")
)

const (
	th32csSnapProcess  = 0x00000002
	processVMRead      = 0x0010
	processQueryInformation = 0x0400
)

type processEntry32 struct {
	Size            uint32
	CntUsage        uint32
	ProcessID       uint32
	DefaultHeapID   uintptr
	ModuleID        uint32
	CntThreads      uint32
	ParentProcessID uint32
	PriorityClass   int32
	Flags           uint32
	ExeFile         [260]uint16
}

// ListProcesses returns all running processes with a non-empty executable
// name, sorted alphabetically.
func ListProcesses() ([]ProcessInfo, error) {
	snapshot, _, err := procCreateToolhelp32Snapshot.Call(th32csSnapProcess, 0)
	if snapshot == uintptr(windows.Handle(0xFFFFFFFF)) {
		return nil, fmt.Errorf("CreateToolhelp32Snapshot failed: %w", err)
	}
	defer procCloseHandle.Call(snapshot)

	var entry processEntry32
	entry.Size = uint32(unsafe.Sizeof(entry))

	ret, _, _ := procProcess32First.Call(snapshot, uintptr(unsafe.Pointer(&entry)))
	if ret == 0 {
		return nil, fmt.Errorf("Process32First failed")
	}

	var processes []ProcessInfo
	for {
		name := windows.UTF16ToString(entry.ExeFile[:])
		if name != "" {
			processes = append(processes, ProcessInfo{
				PID:  entry.ProcessID,
				Name: name,
			})
		}

		ret, _, _ := procProcess32Next.Call(snapshot, uintptr(unsafe.Pointer(&entry)))
		if ret == 0 {
			break
		}
	}

	sort.Slice(processes, func(i, j int) bool {
		return processes[i].Name < processes[j].Name
	})

	return processes, nil
}

// OpenProcessHandle opens a handle to the process with the given PID
// for memory reading (PROCESS_VM_READ | PROCESS_QUERY_INFORMATION).
// Returns windows.InvalidHandle on failure.
func OpenProcessHandle(pid uint32) (windows.Handle, error) {
	h, _, err := procOpenProcess.Call(
		processVMRead|processQueryInformation,
		0,            // bInheritHandle = false
		uintptr(pid),
	)
	if h == 0 {
		return windows.InvalidHandle, fmt.Errorf("OpenProcess(%d) failed: %w", pid, err)
	}
	return windows.Handle(h), nil
}

// CloseProcessHandle closes a process handle obtained from OpenProcessHandle.
func CloseProcessHandle(h windows.Handle) {
	if h != windows.InvalidHandle && h != 0 {
		procCloseHandle.Call(uintptr(h))
	}
}

// ReadProcessUint32 reads a 32-bit unsigned integer from the process
// at the given base address + offset. Returns 0 and an error if the
// read fails (e.g. the process has exited).
func ReadProcessUint32(h windows.Handle, baseAddr uintptr, offset uintptr) (uint32, error) {
	addr := baseAddr + offset
	var val uint32
	var nBytes uintptr
	ret, _, err := procReadProcessMemory.Call(
		uintptr(h),
		addr,
		uintptr(unsafe.Pointer(&val)),
		4, // sizeof(uint32)
		uintptr(unsafe.Pointer(&nBytes)),
	)
	if ret == 0 {
		return 0, fmt.Errorf("ReadProcessMemory(0x%X) failed: %w", addr, err)
	}
	if nBytes != 4 {
		return 0, fmt.Errorf("ReadProcessMemory(0x%X): read %d bytes, want 4", addr, nBytes)
	}
	return val, nil
}


