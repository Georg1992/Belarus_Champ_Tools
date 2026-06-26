package runner

import (
	"context"
	"fmt"
	"image"
	"os"
	"sync"
	"time"

	"github.com/Alia5/VIIPER/viiperclient"
)

const autoPotKeyHold = 1 * time.Millisecond

type AutoPotConfig struct {
	APIAddr     string
	HPThreshold int
	SPThreshold int
	HPKeyVK     int32
	SPKeyVK     int32
	HPEnabled   bool
	SPEnabled   bool
	Log         func(string)
}

func (c *AutoPotConfig) applyDefaults() {
	if c.APIAddr == "" {
		c.APIAddr = DefaultAPIAddr
	}
	if c.Log == nil {
		c.Log = func(string) {}
	}
}

type AutoPotRunner struct {
	cfg AutoPotConfig

	mu      sync.Mutex
	cancel  context.CancelFunc
	done    chan struct{}
	running bool

	liveMu sync.RWMutex
	live   AutoPotConfig
}

func NewAutoPot(cfg AutoPotConfig) *AutoPotRunner {
	cfg.applyDefaults()
	return &AutoPotRunner{cfg: cfg, live: cfg}
}

func (a *AutoPotRunner) Running() bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.running
}

func (a *AutoPotRunner) UpdateSettings(cfg AutoPotConfig) {
	cfg.applyDefaults()
	a.liveMu.Lock()
	a.live = cfg
	a.liveMu.Unlock()
}

func (a *AutoPotRunner) settings() AutoPotConfig {
	a.liveMu.RLock()
	defer a.liveMu.RUnlock()
	return a.live
}

func (a *AutoPotRunner) Start() error {
	a.mu.Lock()
	if a.running {
		a.mu.Unlock()
		return fmt.Errorf("autopot already running")
	}

	cfg := a.settings()
	if cfg.HPEnabled && cfg.HPKeyVK == 0 {
		a.mu.Unlock()
		return fmt.Errorf("HP potion key is not set")
	}
	if cfg.SPEnabled && cfg.SPKeyVK == 0 {
		a.mu.Unlock()
		return fmt.Errorf("SP potion key is not set")
	}

	ctx, cancel := context.WithCancel(context.Background())
	a.cancel = cancel
	a.running = true
	a.done = make(chan struct{})
	a.mu.Unlock()

	go func() {
		defer close(a.done)
		defer func() {
			a.mu.Lock()
			a.running = false
			a.cancel = nil
			a.mu.Unlock()
		}()
		a.run(ctx)
	}()

	return nil
}

func (a *AutoPotRunner) Stop() {
	a.mu.Lock()
	cancel := a.cancel
	a.mu.Unlock()
	if cancel != nil {
		cancel()
	}
}

func (a *AutoPotRunner) Wait() {
	a.mu.Lock()
	done := a.done
	a.mu.Unlock()
	if done != nil {
		<-done
	}
}

func (a *AutoPotRunner) log(msg string) {
	a.cfg.Log(msg)
}

func (a *AutoPotRunner) run(ctx context.Context) {
	a.log("AutoPot starting...")

	api := viiperclient.New(a.cfg.APIAddr)
	if _, err := api.PingCtx(ctx); err != nil {
		a.log(fmt.Sprintf("Connection failed: %v", err))
		return
	}

	busID, createdBus, err := ensureBus(ctx, api, noopLog)
	if err != nil {
		a.log(fmt.Sprintf("Device bus setup failed: %v", err))
		return
	}

	keyStream, keyDev, err := api.AddDeviceAndConnect(ctx, busID, "keyboard", nil)
	if err != nil {
		a.log(fmt.Sprintf("Keyboard setup failed: %v", err))
		cleanupBus(ctx, api, busID, createdBus, noopLog)
		return
	}
	defer keyStream.Close() //nolint:errcheck
	_ = keyDev
	defer func() { _ = keyUp(keyStream) }()

	defer func() {
		cleanupCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		cleanupDevice(cleanupCtx, api, keyStream.BusID, keyStream.DevID, noopLog)
		cleanupBus(cleanupCtx, api, busID, createdBus, noopLog)
	}()

	cfg := a.settings()
	if cfg.HPEnabled {
		a.log(fmt.Sprintf("HP: key %s, trigger below %d%%", KeyName(cfg.HPKeyVK), cfg.HPThreshold))
	}
	if cfg.SPEnabled {
		a.log(fmt.Sprintf("SP: key %s, trigger below %d%%", KeyName(cfg.SPKeyVK), cfg.SPThreshold))
	}

	debugSave := os.Getenv("BAR_SEARCH_DEBUG") != ""
	loggedROI := false
	lastHPLog := ""
	lastSPLog := ""

	for {
		if ctx.Err() != nil {
			a.log("AutoPot stopped")
			return
		}

		img, searchROI, err := CapturePlayerBarSearch()
		if err != nil {
			a.log(fmt.Sprintf("Capture failed: %v", err))
			sleep(ctx, 50*time.Millisecond)
			continue
		}

		if !loggedROI {
			a.log(fmt.Sprintf("Search ROI screen (%d,%d) %dx%d", searchROI.X, searchROI.Y, searchROI.W, searchROI.H))
			loggedROI = true
		}

		hp := FindHPBar(img)
		sp := FindSPBar(img)

		if debugSave {
			_ = SaveBarSearchDebug(img, hp, sp, "bar_search_debug.png")
		}

		hpLog := FormatBarLog("HP", hp)
		if hpLog != lastHPLog {
			a.log(hpLog)
			lastHPLog = hpLog
		}
		spLog := FormatBarLog("SP", sp)
		if spLog != lastSPLog {
			a.log(spLog)
			lastSPLog = spLog
		}

		cfg = a.settings()

		if cfg.HPEnabled && hp.Found && hp.Percent < float64(cfg.HPThreshold) {
			a.healUntil(ctx, keyStream, cfg.HPKeyVK, cfg.HPThreshold, FindHPBar)
			continue
		}

		if cfg.SPEnabled && sp.Found && sp.Percent < float64(cfg.SPThreshold) {
			a.healUntil(ctx, keyStream, cfg.SPKeyVK, cfg.SPThreshold, FindSPBar)
			continue
		}

		sleep(ctx, autoPotKeyHold)
	}
}

func (a *AutoPotRunner) healUntil(ctx context.Context, keyStream *viiperclient.DeviceStream, vk int32, threshold int, read func(image.Image) Bar) {
	for {
		if ctx.Err() != nil {
			return
		}
		img, _, err := CapturePlayerBarSearch()
		if err != nil {
			sleep(ctx, 10*time.Millisecond)
			continue
		}
		bar := read(img)
		if !bar.Found || bar.Percent >= float64(threshold) {
			return
		}
		before := bar.Percent
		if err := tapKey(keyStream, vk); err != nil {
			a.log(fmt.Sprintf("Key %s failed: %v", KeyName(vk), err))
			return
		}
		for {
			if ctx.Err() != nil {
				return
			}
			img, _, err := CapturePlayerBarSearch()
			if err != nil {
				continue
			}
			bar := read(img)
			if !bar.Found || bar.Percent >= float64(threshold) {
				return
			}
			if bar.Percent > before {
				break
			}
		}
	}
}

func tapKey(stream *viiperclient.DeviceStream, vk int32) error {
	if err := keyDown(stream, vk); err != nil {
		return err
	}
	time.Sleep(autoPotKeyHold)
	return keyUp(stream)
}
