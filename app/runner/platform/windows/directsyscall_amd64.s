//go:build windows && amd64
// +build windows,amd64

#include "textflag.h"

// directSyscall executes a raw syscall instruction, completely bypassing
// any user-mode hooks (IAT, inline, or detour) on ntdll or kernel32.
//
// The Windows kernel reads arguments 5+ from the user stack at
// [RSP + 0x28] at the time of the syscall instruction (accounting
// for the return address pushed by CALL and the 32-byte shadow space
// the caller is expected to allocate). We replicate this layout
// manually so the kernel finds the 5th argument in the right place.
//
// Args (Go ABI0 — all on the stack):
//
//	ssn+0(FP)      uint32   — syscall number (System Service Number)
//	handle+8(FP)   uintptr  — HANDLE (process handle)
//	addr+16(FP)    uintptr  — base address to read from
//	buf+24(FP)     uintptr  — output buffer pointer
//	size+32(FP)    uintptr  — number of bytes to read
//	bytesRead+40(FP) uintptr — pointer to SIZE_T for bytes read
//
// Returns:
//
//	ret+48(FP)     uint32   — NTSTATUS (0 = STATUS_SUCCESS)
//
// func directSyscall(ssn uint32, handle, addr, buf, size, bytesRead uintptr) uint32
TEXT ·directSyscall(SB), NOSPLIT, $0
	MOVL    ssn+0(FP), AX           // EAX = SSN
	MOVQ    handle+8(FP), R10       // R10 = ProcessHandle (kernel: 1st arg)
	MOVQ    addr+16(FP), DX         // DX  = BaseAddress (kernel: 2nd arg in RDX)
	MOVQ    buf+24(FP), R8          // R8  = Buffer (kernel: 3rd arg)
	MOVQ    size+32(FP), R9         // R9  = NumberOfBytesToRead (kernel: 4th arg)

	// Allocate 0x30 bytes on the stack, then store the 5th arg
	// (bytesRead pointer) at [SP+0x28]. At syscall time the kernel
	// reads arg 5 from [SP + 0x28] = our stored pointer.
	MOVQ    bytesRead+40(FP), R11   // R11 = bytesRead pointer (AX still holds SSN)
	SUBQ    $0x30, SP               // Make room: 0x28 for kernel + 8 padding
	MOVQ    R11, 0x28(SP)           // 5th arg at kernel-expected offset

	SYSCALL                         // RAX = NTSTATUS

	ADDQ    $0x30, SP               // Restore stack

	MOVL    AX, ret+48(FP)          // Store NTSTATUS return value
	RET
