//go:build windows

package main

import (
	"fmt"
	"sync"
	"syscall"
	"unsafe"

	"github.com/lxn/win"
	"golang.org/x/sys/windows"
)

// Additional Win32 procs not covered by lxn/win.
var (
	ovlUser32   = windows.NewLazySystemDLL("user32.dll")
	ovlGdi32    = windows.NewLazySystemDLL("gdi32.dll")
	ovlKernel32 = windows.NewLazySystemDLL("kernel32.dll")

	procFillRect                = ovlUser32.NewProc("FillRect")
	procSetLayeredWindowAttribs = ovlUser32.NewProc("SetLayeredWindowAttributes")
	procGetModuleHandleW        = ovlKernel32.NewProc("GetModuleHandleW")
	procCreateSolidBrush        = ovlGdi32.NewProc("CreateSolidBrush")
)

// Supplementary Win32 constants.
const (
	ovlFwBold        = 700 // LOGFONT lfWeight FW_BOLD
	ovlLwaAlpha      = 0x00000002
	ovlSwpNoActivate = 0x0010
	ovlSwpShowWindow = 0x0040
)

// overlayClassName is the Win32 window class name for the overlay.
const overlayClassName = "HPSPStatusOverlay"

// overlayInstances maps HWND → *statusOverlay so the WndProc can access it.
var overlayInstances sync.Map

// ovlWndProcFn is the WndProc callback; stored at package level to prevent GC.
var ovlWndProcFn uintptr

// ovlRegisterOnce ensures the window class is registered exactly once.
var ovlRegisterOnce sync.Once

// statusOverlay is a small semi-transparent click-through window that floats
// above the game and displays the last parsed HP/SP values and current mode.
type statusOverlay struct {
	hwnd win.HWND
	font win.HFONT

	mu   sync.Mutex
	text string
	mode string // "OCR", "Pixel-bar", or ""
}

// newStatusOverlay creates and returns a hidden overlay window.
// Must be called on the GUI thread.
func newStatusOverlay() (*statusOverlay, error) {
	o := &statusOverlay{}

	var regErr error
	ovlRegisterOnce.Do(func() {
		ovlWndProcFn = syscall.NewCallback(func(hwnd, msg, wParam, lParam uintptr) uintptr {
			switch uint32(msg) {
			case win.WM_PAINT:
				if v, ok := overlayInstances.Load(win.HWND(hwnd)); ok {
					v.(*statusOverlay).onPaint(win.HWND(hwnd))
				}
				return 0
			case win.WM_ERASEBKGND:
				return 1 // prevent default background erase
			case win.WM_DESTROY:
				overlayInstances.Delete(win.HWND(hwnd))
				return 0
			}
			return win.DefWindowProc(win.HWND(hwnd), uint32(msg), wParam, lParam)
		})

		hInst, _, _ := procGetModuleHandleW.Call(0)
		clsName, _ := syscall.UTF16PtrFromString(overlayClassName)
		wc := win.WNDCLASSEX{
			CbSize:        uint32(unsafe.Sizeof(win.WNDCLASSEX{})),
			LpfnWndProc:   ovlWndProcFn,
			HInstance:     win.HINSTANCE(hInst),
			LpszClassName: clsName,
		}
		if win.RegisterClassEx(&wc) == 0 {
			regErr = fmt.Errorf("RegisterClassEx failed")
		}
	})
	if regErr != nil {
		return nil, regErr
	}

	hInst, _, _ := procGetModuleHandleW.Call(0)
	clsName, _ := syscall.UTF16PtrFromString(overlayClassName)
	title, _ := syscall.UTF16PtrFromString("")

	exStyle := uint32(win.WS_EX_TOPMOST | win.WS_EX_LAYERED | win.WS_EX_TRANSPARENT |
		win.WS_EX_NOACTIVATE | win.WS_EX_TOOLWINDOW)
	hwnd := win.CreateWindowEx(
		exStyle, clsName, title,
		win.WS_POPUP,
		5, 60, 195, 32, // compact: 2 rows + small padding
		0, 0,
		win.HINSTANCE(hInst),
		nil,
	)
	if hwnd == 0 {
		return nil, fmt.Errorf("CreateWindowEx failed")
	}

	// 85% opacity overall.
	procSetLayeredWindowAttribs.Call(uintptr(hwnd), 0, 217, ovlLwaAlpha)

	o.hwnd = hwnd
	overlayInstances.Store(hwnd, o)

	// Bold Consolas 10pt — smaller, less obtrusive.
	lf := win.LOGFONT{LfHeight: -11, LfWeight: ovlFwBold}
	faceUTF16 := windows.StringToUTF16("Consolas")
	copy(lf.LfFaceName[:], faceUTF16)
	o.font = win.CreateFontIndirect(&lf)

	return o, nil
}

// onPaint is called from the WndProc on WM_PAINT.
func (o *statusOverlay) onPaint(hwnd win.HWND) {
	o.mu.Lock()
	text := o.text
	mode := o.mode
	o.mu.Unlock()

	var ps win.PAINTSTRUCT
	hdc := win.BeginPaint(hwnd, &ps)
	if hdc == 0 {
		return
	}
	defer win.EndPaint(hwnd, &ps)

	var rc win.RECT
	win.GetClientRect(hwnd, &rc)

	// Background: near-black.
	bgBrush, _, _ := procCreateSolidBrush.Call(uintptr(win.RGB(15, 15, 20)))
	procFillRect.Call(uintptr(hdc), uintptr(unsafe.Pointer(&rc)), bgBrush)
	win.DeleteObject(win.HGDIOBJ(bgBrush))

	if text == "" && mode == "" {
		return
	}

	oldFont := win.SelectObject(hdc, win.HGDIOBJ(o.font))
	win.SetBkMode(hdc, 1) // TRANSPARENT

	// Row 1: HP/SP values (white), compact top.
	if text != "" {
		win.SetTextColor(hdc, win.RGB(240, 240, 240))
		textUTF16, _ := syscall.UTF16PtrFromString(text)
		textLen := int32(len([]rune(text)))
		win.TextOut(hdc, 4, 2, textUTF16, textLen)
	}

	// Row 2: mode message, compact below the values.
	// Alert modes (Dead, pots-ended) highlighted in orange; normal modes in dim grey.
	if mode != "" {
		modeText := "[" + mode + "]"
		isAlert := mode != "OCR" && mode != "Pixel-bar" && mode != "Searching..." && mode != "Stopped"
		if isAlert {
			win.SetTextColor(hdc, win.RGB(255, 160, 70))
		} else {
			win.SetTextColor(hdc, win.RGB(180, 180, 190))
		}
		modeUTF16, _ := syscall.UTF16PtrFromString(modeText)
		modeLen := int32(len([]rune(modeText)))
		win.TextOut(hdc, 4, 17, modeUTF16, modeLen)
	}

	win.SelectObject(hdc, oldFont)
}

// Update stores the latest HP/SP values and repositions the window just below
// the status panel. Safe to call from any goroutine.
//
// Sentinel: when hp < 0 or sp < 0 the overlay shows an error message
// (pixel-bar fallback is active, no OCR data available).
//
// Otherwise, hpMax > 0 means OCR absolute values (HP/nnn); hpMax == 0
// is a bare fallback with no max known.
//
// The last 4 params are panelX, panelY, panelW, panelH from OCR detection.
// The overlay is positioned below the full panel (panelY+panelH+3).
func (o *statusOverlay) Update(hp, hpMax, sp, spMax, panelX, panelY, panelW, panelH int) {
	var text string
	if hp < 0 || sp < 0 {
		text = "error: Pixelsearch is used"
	} else if hpMax > 0 && spMax > 0 {
		text = fmt.Sprintf("HP %d/%d  SP %d/%d", hp, hpMax, sp, spMax)
	} else if hpMax > 0 {
		text = fmt.Sprintf("HP %d/%d", hp, hpMax)
	} else {
		text = fmt.Sprintf("HP %d  SP %d", hp, sp)
	}

	o.mu.Lock()
	o.text = text
	o.mu.Unlock()

	// Reposition only when we have valid panel coordinates (OCR reader).
	// Pixel-bar reader passes zeros — skip repositioning, just repaint.
	if panelW > 0 && panelH > 0 {
		// Compact overlay below the panel: fixed width (195px) fits all text,
		// fixed height (32px) for 2 rows + small padding.
		win.SetWindowPos(
			o.hwnd, win.HWND_TOPMOST,
			int32(panelX), int32(panelY+panelH+3),
			195, 32,
			ovlSwpNoActivate|ovlSwpShowWindow,
		)
	}
	win.InvalidateRect(o.hwnd, nil, true)
}

// SetMode updates the mode label shown in the overlay (e.g. "OCR" or
// "Pixel-bar"). Pass "" to hide the label. Safe from any goroutine.
func (o *statusOverlay) SetMode(mode string) {
	o.mu.Lock()
	o.mode = mode
	o.mu.Unlock()
	win.InvalidateRect(o.hwnd, nil, true)
}

// ShowStopped clears the HP/SP text and sets the mode label to "Stopped"
// so the overlay displays just "[Stopped]" without stale values.
func (o *statusOverlay) ShowStopped() {
	o.mu.Lock()
	o.text = ""
	o.mode = "Stopped"
	o.mu.Unlock()
	win.InvalidateRect(o.hwnd, nil, true)
}

// Hide hides the overlay without destroying it.
func (o *statusOverlay) Hide() {
	win.ShowWindow(o.hwnd, win.SW_HIDE)
}

// Destroy releases the overlay window and its GDI resources.
func (o *statusOverlay) Destroy() {
	if o.font != 0 {
		win.DeleteObject(win.HGDIOBJ(o.font))
		o.font = 0
	}
	if o.hwnd != 0 {
		win.DestroyWindow(o.hwnd)
		o.hwnd = 0
	}
}
