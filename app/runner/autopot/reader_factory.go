package autopot

import (
	"belarus-champ-tools/runner/autopot/statusui"
	win "belarus-champ-tools/runner/platform/windows"
)

// ReaderFactory constructs BarReader instances based on the provided config.
// It encapsulates the decision tree for choosing between address, OCR, and
// pixel readers, making the system open for extension (new reader types can
// be added without modifying the orchestrator).
type ReaderFactory struct {
	settings func() AutoPotConfig // live config for runtime threshold lookups
	hpStab   *BarStabilizer
	spStab   *BarStabilizer
}

// NewReaderFactory creates a factory for the given settings getter and stabilizers.
// The settings function provides live access to the config (thresholds can change
// via UpdateSettings mid-run).
func NewReaderFactory(settings func() AutoPotConfig, hpStab, spStab *BarStabilizer) *ReaderFactory {
	return &ReaderFactory{
		settings: settings,
		hpStab:   hpStab,
		spStab:   spStab,
	}
}

// IsAddressMode reports whether the factory will produce an address reader.
func (f *ReaderFactory) IsAddressMode() bool {
	cfg := f.settings()
	return cfg.IsAddressMode()
}

// Build creates the primary BarReader, the pixel fallback, and the OCR
// reader (nil if OCR is unavailable or in address mode).
//
// Returns:
//   - primary: the active BarReader
//   - fallback: pixel reader for OCR→pixel fallback; nil in address mode
//   - ocr: OCR reader for pixel→OCR recovery; nil if unavailable
func (f *ReaderFactory) Build() (primary BarReader, fallback *pixelBarReader, ocr *statusUIReader) {
	cfg := f.settings()
	if cfg.IsAddressMode() {
		reader, err := f.buildAddressReader(cfg)
		if err != nil {
			cfg.Core.Log("autopot: " + err.Error() + " — falling back to Visual mode")
			return f.buildVisualReaders(cfg)
		}
		setMode(cfg.Core.OnStatusUIMode, "Address reading")
		return reader, nil, nil
	}
	return f.buildVisualReaders(cfg)
}

// buildVisualReaders creates pixel + optional OCR readers for visual mode.
func (f *ReaderFactory) buildVisualReaders(cfg AutoPotConfig) (primary BarReader, fallback *pixelBarReader, ocr *statusUIReader) {
	pixel := f.buildPixelReader(cfg)
	if ocr, ok := f.tryBuildOCRReader(cfg); ok {
		setMode(cfg.Core.OnStatusUIMode, "Searching...")
		return ocr, pixel, ocr
	}
	setMode(cfg.Core.OnStatusUIMode, "Pixelsearch")
	if cfg.Core.OnStatusParsed != nil {
		cfg.Core.OnStatusParsed(pixelModeSentinel, 0, pixelModeSentinel, 0, 0, 0, 0, 0)
	}
	return pixel, pixel, nil
}

func (f *ReaderFactory) buildPixelReader(cfg AutoPotConfig) *pixelBarReader {
	return &pixelBarReader{
		hpStab:   f.hpStab,
		spStab:   f.spStab,
		log:      cfg.Core.Log,
		onParsed: cfg.Core.OnStatusParsed,
	}
}

func (f *ReaderFactory) tryBuildOCRReader(cfg AutoPotConfig) (*statusUIReader, bool) {
	pipeline, err := statusui.NewDefaultPipeline()
	if err != nil {
		return nil, false
	}
	return &statusUIReader{
		poller:       statusui.NewStripPoller(pipeline),
		onModeChange: cfg.Core.OnStatusUIMode,
		onParsed:     cfg.Core.OnStatusParsed,
		log:          cfg.Core.Log,
		coreSettings: func() CoreConfig { return f.settings().Core },
	}, true
}

func (f *ReaderFactory) buildAddressReader(cfg AutoPotConfig) (*addressReader, error) {
	baseAddr, err := win.GetProcessBaseAddr(cfg.Address.ProcessPID)
	if err != nil {
		return nil, err
	}
	return &addressReader{
		pid:          cfg.Address.ProcessPID,
		profile:      cfg.Address.Profile,
		processTitle: cfg.Address.ProcessTitle,
		moduleBase:   baseAddr,
		log:          cfg.Core.Log,
		thresholdFn:  func() (hpThresh, spThresh int) { c := f.settings().Core; return c.HPThreshold, c.SPThreshold },
		onParsed:     cfg.Core.OnStatusParsed,
		onModeChange: cfg.Core.OnStatusUIMode,
	}, nil
}
