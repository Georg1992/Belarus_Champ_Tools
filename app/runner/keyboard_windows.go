//go:build windows

package runner

import windows "belarus-champ-tools/runner/platform/windows"

func init() {
	PhysicalKeyDown = windows.WinPhysicalKeyDown
	PollKeyToggle = windows.WinPollKeyToggle
}
