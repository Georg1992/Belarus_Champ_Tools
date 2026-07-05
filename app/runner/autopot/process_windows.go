//go:build windows

package autopot

import windows "belarus-champ-tools/runner/platform/windows"

func init() {
	GetProcessBaseAddr = windows.GetProcessBaseAddr
}
