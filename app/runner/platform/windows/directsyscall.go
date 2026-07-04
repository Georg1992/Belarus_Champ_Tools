//go:build windows && amd64
// +build windows,amd64

package runner

import (
	"encoding/binary"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"unsafe"
)

// directSyscall executes a raw syscall instruction, completely bypassing
// any user-mode hooks on ntdll or kernel32.
//
// Implemented in directsyscall_amd64.s.
//
//go:noescape
func directSyscall(ssn uint32, handle, addr, buf, size, bytesRead uintptr) uint32

var (
	resolveSSNOnce sync.Once
	cachedSSN      uint32
	ssnErr         error
)

// getNtReadVirtualMemorySSN resolves the System Service Number (SSN) for
// NtReadVirtualMemory by reading the CLEAN copy of ntdll.dll from disk.
//
// Anti-cheat systems like Gepard may hook the in-memory copy of ntdll
// deeply enough to overwrite the entire NtReadVirtualMemory stub (mov
// r10, rcx; mov eax, SSN; syscall; ret). However, the file on disk is
// never modified. We parse the PE headers from the file, find the
// NtReadVirtualMemory export, and read the raw stub bytes from the
// file at the correct offset to extract the SSN.
//
// The pattern we look for:
//
//	B8 XX XX XX XX    mov eax, SSN     (5 bytes)
//	0F 05             syscall           (2 bytes)
func getNtReadVirtualMemorySSN() (uint32, error) {
	resolveSSNOnce.Do(func() {
		cachedSSN, ssnErr = resolveSSNFromDisk()
	})

	if ssnErr != nil {
		return 0, ssnErr
	}
	return cachedSSN, nil
}

// resolveSSNFromDisk reads ntdll.dll from the file system and parses the
// PE export table to find the NtReadVirtualMemory stub and extract its SSN.
func resolveSSNFromDisk() (uint32, error) {
	// Determine the path to ntdll.dll.
	systemRoot := os.Getenv("SystemRoot")
	if systemRoot == "" {
		systemRoot = `C:\Windows`
	}
	ntdllPath := filepath.Join(systemRoot, "System32", "ntdll.dll")

	data, err := os.ReadFile(ntdllPath)
	if err != nil {
		return 0, fmt.Errorf("cannot read %s: %w", ntdllPath, err)
	}

	if len(data) < 64 || data[0] != 'M' || data[1] != 'Z' {
		return 0, fmt.Errorf("%s: invalid DOS header", ntdllPath)
	}

	eLfanew := binary.LittleEndian.Uint32(data[0x3C:0x40])
	if int(eLfanew+4) >= len(data) || data[eLfanew] != 'P' || data[eLfanew+1] != 'E' {
		return 0, fmt.Errorf("%s: invalid PE signature", ntdllPath)
	}

	coff := eLfanew + 4
	numSections := binary.LittleEndian.Uint16(data[coff+2 : coff+4])
	optHeaderSize := binary.LittleEndian.Uint16(data[coff+16 : coff+18])
	optHeader := coff + 20

	magic := binary.LittleEndian.Uint16(data[optHeader : optHeader+2])
	var dataDirOffset uint32
	switch magic {
	case 0x10B: // PE32
		dataDirOffset = uint32(optHeader) + 96
	case 0x20B: // PE32+
		dataDirOffset = uint32(optHeader) + 112
	default:
		return 0, fmt.Errorf("%s: unknown PE magic 0x%04X", ntdllPath, magic)
	}

	exportRVA := binary.LittleEndian.Uint32(data[dataDirOffset : dataDirOffset+4])
	exportSize := binary.LittleEndian.Uint32(data[dataDirOffset+4 : dataDirOffset+8])
	if exportRVA == 0 || exportSize == 0 {
		return 0, fmt.Errorf("%s: no export directory", ntdllPath)
	}

	// Build section lookup table for RVA→file offset conversion.
	sections := uint32(optHeader) + uint32(optHeaderSize)
	rvaToOffset := func(rva uint32) uint32 {
		for i := uint32(0); i < uint32(numSections); i++ {
			sec := sections + i*40
			scVa := binary.LittleEndian.Uint32(data[sec+12 : sec+16])
			scRawSize := binary.LittleEndian.Uint32(data[sec+16 : sec+20])
			scRawPtr := binary.LittleEndian.Uint32(data[sec+20 : sec+24])
			if rva >= scVa && rva < scVa+scRawSize {
				return rva - scVa + scRawPtr
			}
		}
		return 0
	}

	exportFileOff := rvaToOffset(exportRVA)
	if exportFileOff == 0 {
		return 0, fmt.Errorf("%s: export directory RVA 0x%X not found in any section", ntdllPath, exportRVA)
	}

	// Parse IMAGE_EXPORT_DIRECTORY.
	numNames := binary.LittleEndian.Uint32(data[exportFileOff+0x18 : exportFileOff+0x1C])
	addrOfFuncs := binary.LittleEndian.Uint32(data[exportFileOff+0x1C : exportFileOff+0x20])
	addrOfNames := binary.LittleEndian.Uint32(data[exportFileOff+0x20 : exportFileOff+0x24])
	addrOfOrdinals := binary.LittleEndian.Uint32(data[exportFileOff+0x24 : exportFileOff+0x28])

	funcOff := rvaToOffset(addrOfFuncs)
	nameOff := rvaToOffset(addrOfNames)
	ordOff := rvaToOffset(addrOfOrdinals)
	if funcOff == 0 || nameOff == 0 || ordOff == 0 {
		return 0, fmt.Errorf("%s: cannot resolve export tables in file", ntdllPath)
	}

	// Walk the export name table looking for "NtReadVirtualMemory".
	target := "NtReadVirtualMemory"
	for i := uint32(0); i < numNames; i++ {
		nameRVA := binary.LittleEndian.Uint32(data[nameOff+i*4 : nameOff+i*4+4])
		nameFileOff := rvaToOffset(nameRVA)
		if nameFileOff == 0 {
			continue
		}

		// Read null-terminated string from the file.
		nameBytes := data[nameFileOff:]
		end := 0
		for end < len(nameBytes) && nameBytes[end] != 0 {
			end++
		}
		if string(nameBytes[:end]) != target {
			continue
		}

		ordinal := binary.LittleEndian.Uint16(data[ordOff+i*2 : ordOff+i*2+2])
		funcRVA := binary.LittleEndian.Uint32(data[funcOff+uint32(ordinal)*4 : funcOff+uint32(ordinal)*4+4])

		// Reject forwarded exports (RVA inside the export directory).
		if funcRVA >= exportRVA && funcRVA < exportRVA+exportSize {
			return 0, fmt.Errorf("%s: NtReadVirtualMemory is a forwarded export", ntdllPath)
		}

		funcFileOff := rvaToOffset(funcRVA)
		if funcFileOff == 0 {
			return 0, fmt.Errorf("%s: function RVA 0x%X not in any section", ntdllPath, funcRVA)
		}

		// Read up to 128 bytes of the function stub from the clean file.
		maxBytes := uint32(128)
		if funcFileOff+maxBytes > uint32(len(data)) {
			maxBytes = uint32(len(data)) - funcFileOff
		}
		stub := data[funcFileOff : funcFileOff+maxBytes]

		// Scan for 0F 05 (syscall), then walk backward up to 20 bytes
		// looking for B8 (mov eax, imm32 = SSN).
		//
		// On older Windows the delta is 5 bytes (mov r10, rcx; mov eax, SSN; syscall; ret).
		// On Windows 11 24H2+ there's a test/je mitigation between SSN and syscall
		// (test byte ptr [0x7FFE0308], 1; jne +3) making the delta 15 bytes.
		// Searching 20 bytes backward covers both cases with margin.
		for j := 2; j < len(stub); j++ {
			if stub[j-2] == 0x0F && stub[j-1] == 0x05 {
				searchStart := j - 2 - 20
				if searchStart < 0 {
					searchStart = 0
				}
				for k := j - 2 - 1; k >= int(searchStart); k-- {
					if stub[k] == 0xB8 && k+5 <= len(stub) {
						return binary.LittleEndian.Uint32(stub[k+1 : k+5]), nil
					}
				}
			}
		}

		// Hex dump for debugging if all strategies fail.
		hexDump := ""
		for _, b := range stub[:min(64, len(stub))] {
			hexDump += fmt.Sprintf("%02X ", b)
		}
		return 0, fmt.Errorf("SSN pattern not found. First 64 bytes at file offset 0x%X (RVA 0x%X): %s", funcFileOff, funcRVA, hexDump)
	}

	return 0, fmt.Errorf("%s: export '%s' not found in name table", ntdllPath, target)
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// ReadProcessUint32ViaSyscall reads a 32-bit value from the target process
// using a direct syscall (bypassing ntdll and kernel32 hooks entirely).
//
// Opens a handle, reads, and closes — matching the user's AHK pattern
// to avoid stale-handle issues. The SSN is resolved once and cached.
func ReadProcessUint32ViaSyscall(pid uint32, addr uintptr) (uint32, error) {
	ssn, err := getNtReadVirtualMemorySSN()
	if err != nil {
		return 0, err
	}

	h, _, errno := procOpenProcess.Call(processVMRead, 0, uintptr(pid))
	if h == 0 {
		return 0, fmt.Errorf("OpenProcess(%d) failed: %w", pid, errno)
	}
	defer procCloseHandle.Call(h)

	var val uint32
	var bytesRead uintptr

	status := directSyscall(ssn, h, addr, uintptr(unsafe.Pointer(&val)), 4, uintptr(unsafe.Pointer(&bytesRead)))
	if status != 0 {
		return 0, fmt.Errorf("NtReadVirtualMemory(0x%X) via syscall failed: NTSTATUS 0x%08X", addr, status)
	}
	if bytesRead != 4 {
		return 0, fmt.Errorf("NtReadVirtualMemory(0x%X) via syscall: read %d bytes, want 4", addr, bytesRead)
	}
	return val, nil
}
