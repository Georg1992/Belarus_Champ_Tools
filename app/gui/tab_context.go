//go:build windows

package main

import (
	"fmt"
	"os"
	"runtime/debug"
	"sync"

	"belarus-champ-tools/runner"
	"github.com/lxn/walk"
)

// tabContext provides shared dependencies to all tab controllers.
// Each controller receives this at construction and uses it to access
// the VIIPER session, logging, overlay, and key-binding infrastructure
// without coupling back to guiApp directly.
type tabContext struct {
	mu            *sync.Mutex
	window        *walk.MainWindow
	getSession    func() runner.InputSession // returns current session under mu
	appendLog     func(string)
	overlay       *statusOverlay
	isStarted     func() bool
	isViiperReady func() bool
	// unsetBinding is set after all controllers are constructed so each
	// controller's unsetBinding method can be called in priority order.
	unsetBinding func(vk int32)
	bindActive   *bool
}

// guiLog wraps a log function in window.Synchronize for safe cross-goroutine use.
func (ctx *tabContext) guiLog(fn func(string)) func(string) {
	return func(s string) {
		ctx.window.Synchronize(func() { fn(s) })
	}
}

// session is a convenience accessor that reads the session under mu.
func (ctx *tabContext) session() runner.InputSession {
	if ctx.getSession != nil {
		return ctx.getSession()
	}
	return nil
}

// bindKeyFlow runs the "user presses a key to bind" interaction shared
// by all four tabs. Controllers call this via their ctx reference.
//
// Pipeline:
//  1. gate is checked synchronously. Side-effects (e.g. setting a
//     binding-slot index so concurrent binds are rejected) are OK
//     inside gate — they happen before the goroutine spawns. `false`
//     bails without spawning.
//  2. prompt is logged once.
//  3. A goroutine waits for a keypress with runner.WaitForKeyPress
//     (runner.KeyBindTimeout).
//  4. The result is dispatched back to the UI thread via
//     window.Synchronize, where:
//     * timeout → "Key bind timed out"
//     * unsupported VK (VKToHID fails) → "Key <name> is not supported"
//     * otherwise onPress(vk) runs.
//  5. cleanup runs (off-UI) once the goroutine exits, regardless of
//     outcome — use it to un-register binding state.
//  6. reenable runs on the UI thread after cleanup, refreshing bind
//     button enabled state.
//
// Returns true if a binding goroutine was spawned.
func (ctx *tabContext) bindKeyFlow(
	gate func() bool,
	prompt string,
	cleanup func(),
	reenable func(),
	onPress func(vk int32),
) bool {
	if !gate() {
		return false
	}
	ctx.appendLog(prompt)
	go func() {
		defer func() {
			if r := recover(); r != nil {
				_, _ = fmt.Fprintf(os.Stderr, "PANIC in bindKeyFlow: %v\n%s\n", r, debug.Stack())
				cleanup()
			}
		}()
		defer func() {
			cleanup()
			ctx.window.Synchronize(func() {
				reenable()
			})
		}()
		vk, ok := runner.WaitForKeyPress(runner.KeyBindTimeout)
		ctx.window.Synchronize(func() {
			if !ok {
				ctx.appendLog("Key bind timed out")
				return
			}
			if _, hidOK := runner.VKToHID(vk); !hidOK {
				ctx.appendLog(fmt.Sprintf("Key %s is not supported", runner.KeyName(vk)))
				return
			}
			onPress(vk)
		})
	}()
	return true
}
