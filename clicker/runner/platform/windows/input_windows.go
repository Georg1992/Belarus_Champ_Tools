//go:build windows

package runner

import (
	"golang.org/x/sys/windows"
)

var (
	user32               = windows.NewLazySystemDLL("user32.dll")
	procGetAsyncKeyState = user32.NewProc("GetAsyncKeyState")
	procGetDC            = user32.NewProc("GetDC")
	procReleaseDC        = user32.NewProc("ReleaseDC")
	procGetSystemMetrics = user32.NewProc("GetSystemMetrics")
)

const escapeVK = 0x1B

func PollKeyToggle(wasDown *bool, vk int32) bool {
	down := PhysicalKeyDown(vk)
	toggled := down && !*wasDown
	*wasDown = down
	return toggled
}

func PhysicalKeyDown(vk int32) bool {
	ret, _, _ := procGetAsyncKeyState.Call(uintptr(vk))
	return ret&0x8000 != 0
}

func ActiveTrigger(vks []int32) (int32, bool) {
	for _, vk := range vks {
		if PhysicalKeyDown(vk) {
			return vk, true
		}
	}
	return 0, false
}

func TriggerHeld(triggerVKs []int32) bool {
	_, held := ActiveTrigger(triggerVKs)
	return held
}
