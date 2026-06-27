//go:build windows

package main

import (
	"math"
	"sync"

	"github.com/lxn/walk"
)

const (
	keyChainKeyFieldWidth   = 56
	keyChainDelayFieldWidth = 80
	keyChainStepWidth       = keyChainDelayFieldWidth
	keyChainFieldHeight     = 22
	keyChainDownHeight      = 18
	keyChainLinkWidth       = 20
)

var keyChainArrowColor = walk.RGB(110, 110, 110)

var (
	keyChainSurfaceBrush walk.Brush
	keyChainSurfaceOnce  sync.Once
)

func keyChainSurface() walk.Brush {
	keyChainSurfaceOnce.Do(func() {
		keyChainSurfaceBrush, _ = walk.NewSystemColorBrush(walk.SysColorBtnFace)
	})
	return keyChainSurfaceBrush
}

func applyKeyChainSurface(w walk.Window) {
	w.SetBackground(keyChainSurface())
}

func keyChainStepHeight() int {
	return keyChainFieldHeight + keyChainDownHeight + keyChainFieldHeight
}

func keyChainKeyCenterY(bounds walk.Rectangle) int {
	return int(float64(bounds.Height) * (float64(keyChainFieldHeight) / 2.0) / float64(keyChainStepHeight()))
}

func keyChainDelayCenterY(bounds walk.Rectangle) int {
	top := float64(keyChainFieldHeight + keyChainDownHeight)
	return int(float64(bounds.Height) * (top + float64(keyChainFieldHeight)/2.0) / float64(keyChainStepHeight()))
}

func newKeyChainPen() (walk.Pen, error) {
	return walk.NewCosmeticPen(walk.PenSolid, keyChainArrowColor)
}

func drawArrowHead(canvas *walk.Canvas, pen walk.Pen, tip, from walk.Point) error {
	dx := float64(tip.X - from.X)
	dy := float64(tip.Y - from.Y)
	length := math.Hypot(dx, dy)
	if length < 1 {
		return nil
	}
	dx /= length
	dy /= length
	perpX := -dy
	perpY := dx
	size := 3.5
	p1 := walk.Point{
		X: tip.X - int(dx*size+perpX*size*0.6),
		Y: tip.Y - int(dy*size+perpY*size*0.6),
	}
	p2 := walk.Point{
		X: tip.X - int(dx*size-perpX*size*0.6),
		Y: tip.Y - int(dy*size-perpY*size*0.6),
	}
	if err := canvas.DrawLinePixels(pen, tip, p1); err != nil {
		return err
	}
	return canvas.DrawLinePixels(pen, tip, p2)
}

func drawLineArrow(canvas *walk.Canvas, pen walk.Pen, from, to walk.Point) error {
	if err := canvas.DrawLinePixels(pen, from, to); err != nil {
		return err
	}
	return drawArrowHead(canvas, pen, to, from)
}

func fillKeyChainSurface(canvas *walk.Canvas, bounds walk.Rectangle) error {
	return canvas.FillRectanglePixels(keyChainSurface(), bounds)
}

func initKeyChainArrowWidget(w *walk.CustomWidget) {
	applyKeyChainSurface(w)
	w.SetPaintMode(walk.PaintNoErase)
	w.SetInvalidatesOnResize(true)
}

func newKeyChainDownArrow(parent walk.Container) (*walk.CustomWidget, error) {
	w, err := walk.NewCustomWidgetPixels(parent, 0, func(canvas *walk.Canvas, bounds walk.Rectangle) error {
		if err := fillKeyChainSurface(canvas, bounds); err != nil {
			return err
		}

		pen, err := newKeyChainPen()
		if err != nil {
			return err
		}
		defer pen.Dispose()

		cx := bounds.Width / 2
		from := walk.Point{X: cx, Y: 0}
		to := walk.Point{X: cx, Y: bounds.Height - 1}
		return drawLineArrow(canvas, pen, from, to)
	})
	if err != nil {
		return nil, err
	}
	initKeyChainArrowWidget(w)
	return w, nil
}

func newKeyChainStepLink(parent walk.Container) (*walk.CustomWidget, error) {
	w, err := walk.NewCustomWidgetPixels(parent, 0, func(canvas *walk.Canvas, bounds walk.Rectangle) error {
		if err := fillKeyChainSurface(canvas, bounds); err != nil {
			return err
		}

		pen, err := newKeyChainPen()
		if err != nil {
			return err
		}
		defer pen.Dispose()

		w := bounds.Width
		keyY := keyChainKeyCenterY(bounds)
		delayY := keyChainDelayCenterY(bounds)
		midX := w * 2 / 5

		if err := canvas.DrawLinePixels(pen, walk.Point{X: 0, Y: keyY}, walk.Point{X: w, Y: keyY}); err != nil {
			return err
		}

		points := []walk.Point{
			{X: 0, Y: delayY},
			{X: midX, Y: delayY},
			{X: midX, Y: keyY},
			{X: w, Y: keyY},
		}
		if err := canvas.DrawPolylinePixels(pen, points); err != nil {
			return err
		}
		return drawArrowHead(canvas, pen, walk.Point{X: w, Y: keyY}, walk.Point{X: midX, Y: keyY})
	})
	if err != nil {
		return nil, err
	}
	initKeyChainArrowWidget(w)
	return w, nil
}
