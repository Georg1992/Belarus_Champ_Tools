// Package timing holds the canonical timing constants and the cancellable
// Sleep helper used by every runner. Centralizing here avoids the
// duplication that existed across runner/timing.go and runner/autopot/.
package timing

import (
	"context"
	"fmt"
	"strings"
	"time"
)

const (
	PollInterval      = 10 * time.Millisecond
	MinPollWait       = time.Millisecond
	CaptureRetryDelay = 50 * time.Millisecond
	KeyTapHold        = 1 * time.Millisecond
	// MouseClickHold is the LMB press duration for clicker mouse clicks.
	// Kept separate from the clicker inter-fire DelayMs.
	MouseClickHold = KeyTapHold
	KeyBindTimeout = 5 * time.Second
	SessionCloseWait = 10 * time.Second
)

// Virtual-key codes for the start/stop toggle watcher.
const (
	VKEnd = 0x23 // VK_END
	VKF12 = 0x7B // VK_F12
)

// ToggleVKs are the virtual-key codes for the stop/start toggle watcher.
var ToggleVKs = []int32{VKEnd, VKF12}

// ToggleKeyLabel returns a short UI string listing ToggleVKs by name.
func ToggleKeyLabel() string {
	names := make([]string, 0, len(ToggleVKs))
	for _, vk := range ToggleVKs {
		switch vk {
		case VKEnd:
			names = append(names, "End")
		case VKF12:
			names = append(names, "F12")
		default:
			names = append(names, fmt.Sprintf("VK_0x%02X", vk))
		}
	}
	return strings.Join(names, " / ")
}

// DefaultAPIAddr is the default address of the embedded VIIPER API server.
// Port 3242 verified at runtime (2026-07-02): "API listening addr=[::]:3242".
// Format is host:port — viiperclient passes this directly to net.Dial("tcp", addr).
const DefaultAPIAddr = "127.0.0.1:3242"

// Sleep sleeps for d, returning early if ctx is canceled.
func Sleep(ctx context.Context, d time.Duration) {
	if d <= 0 {
		select {
		case <-ctx.Done():
		default:
		}
		return
	}
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
	case <-t.C:
	}
}
