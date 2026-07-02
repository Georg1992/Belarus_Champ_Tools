//go:build windows

package main

import (
	"context"
	"time"

	"belarus-champ-tools/runner"

	"github.com/Alia5/VIIPER/viiperclient"
)

// viiperMonitor periodically pings the VIIPER server and calls
// onStatusChange whenever the active/inactive state transitions.
type viiperMonitor struct {
	cancel context.CancelFunc
	done   chan struct{}
}

// startViiperMonitor launches a goroutine that pings the VIIPER API
// every 2 seconds. onStatusChange is called on the monitor's goroutine
// — the caller must marshal UI updates to the GUI thread.
func startViiperMonitor(ctx context.Context, onStatusChange func(active bool)) *viiperMonitor {
	monitorCtx, cancel := context.WithCancel(ctx)
	m := &viiperMonitor{
		cancel: cancel,
		done:   make(chan struct{}),
	}

	go func() {
		defer close(m.done)
		addr := runner.DefaultAPIAddr
		api := viiperclient.New(addr)
		wasActive := false

		for monitorCtx.Err() == nil {
			pingCtx, pingCancel := context.WithTimeout(monitorCtx, 2*time.Second)
			_, err := api.PingCtx(pingCtx)
			pingCancel()

			active := err == nil
			if active != wasActive {
				wasActive = active
				onStatusChange(active)
			}

			select {
			case <-monitorCtx.Done():
				return
			case <-time.After(2 * time.Second):
			}
		}
	}()

	return m
}

// stop stops the monitor and waits for the goroutine to finish.
func (m *viiperMonitor) stop() {
	if m == nil {
		return
	}
	m.cancel()
	<-m.done
}
