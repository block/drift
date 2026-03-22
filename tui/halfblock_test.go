package tui

import (
	"image"
	"image/color"
	"strings"
	"testing"
)

func solidImage(w, h int, c color.NRGBA) *image.NRGBA {
	img := image.NewNRGBA(image.Rect(0, 0, w, h))
	for i := 0; i < len(img.Pix); i += 4 {
		img.Pix[i] = c.R
		img.Pix[i+1] = c.G
		img.Pix[i+2] = c.B
		img.Pix[i+3] = c.A
	}
	return img
}

func TestScaleForTerminal_Basic(t *testing.T) {
	// 100x100 image at 50 columns wide should give 50x50, padded to even height.
	dstW, dstH := scaleForTerminal(100, 100, 50, 0)
	if dstW != 50 {
		t.Errorf("dstW = %d, want 50", dstW)
	}
	if dstH != 50 {
		t.Errorf("dstH = %d, want 50", dstH)
	}
}

func TestScaleForTerminal_OddHeight(t *testing.T) {
	// 100x50 at 100 wide -> 100x50 (even). 100x51 at 100 wide -> 100x52 (padded).
	_, dstH := scaleForTerminal(100, 51, 100, 0)
	if dstH%2 != 0 {
		t.Errorf("dstH = %d, want even number", dstH)
	}
}

func TestScaleForTerminal_MaxRows(t *testing.T) {
	// Tall image (100x400) at 100 wide -> 100x400, but maxRows=50 caps to 100px height.
	dstW, dstH := scaleForTerminal(100, 400, 100, 50)
	if dstH > 100 {
		t.Errorf("dstH = %d, want <= 100 (50 rows * 2)", dstH)
	}
	if dstW <= 0 {
		t.Errorf("dstW = %d, want > 0", dstW)
	}
}

func TestScaleForTerminal_MinimumSize(t *testing.T) {
	// Very large image scaled to tiny target should still have minimum dimensions.
	dstW, dstH := scaleForTerminal(10000, 10000, 1, 0)
	if dstW < 1 || dstH < 1 {
		t.Errorf("dimensions = %dx%d, want >= 1x1", dstW, dstH)
	}
}

func TestRenderHalfBlock_NilImage(t *testing.T) {
	if got := renderHalfBlock(nil, 40, 0); got != "" {
		t.Errorf("nil image returned %q, want empty", got)
	}
}

func TestRenderHalfBlock_ZeroWidth(t *testing.T) {
	img := solidImage(10, 10, color.NRGBA{R: 255, A: 255})
	if got := renderHalfBlock(img, 0, 0); got != "" {
		t.Errorf("zero width returned %q, want empty", got)
	}
}

func TestRenderHalfBlock_ProducesOutput(t *testing.T) {
	img := solidImage(4, 4, color.NRGBA{R: 128, G: 64, B: 32, A: 255})
	got := renderHalfBlock(img, 4, 0)

	if got == "" {
		t.Fatal("expected non-empty output")
	}
	if !strings.Contains(got, "▀") {
		t.Error("expected half-block characters in output")
	}
	// Should contain ANSI color codes.
	if !strings.Contains(got, "\x1b[38;2;") {
		t.Error("expected truecolor foreground escape codes")
	}
	if !strings.Contains(got, "\x1b[48;2;") {
		t.Error("expected truecolor background escape codes")
	}
}

func TestRenderHalfBlock_ResetPerLine(t *testing.T) {
	img := solidImage(2, 4, color.NRGBA{R: 255, A: 255})
	got := renderHalfBlock(img, 2, 0)

	// Each line should start with a reset to prevent color bleed.
	lines := strings.Split(got, "\n")
	for i, line := range lines {
		if !strings.HasPrefix(line, "\x1b[0m") {
			t.Errorf("line %d doesn't start with reset: %q", i, line[:min(20, len(line))])
		}
	}
}

func TestRenderHalfBlockOverlay_NilOverlay(t *testing.T) {
	base := solidImage(4, 4, color.NRGBA{R: 128, A: 255})
	got := renderHalfBlockOverlay(base, nil, 4, 0)

	// Should still render the base image without crashing.
	if got == "" {
		t.Fatal("expected non-empty output for nil overlay")
	}
}

func TestRenderHalfBlockOverlay_WithOverlay(t *testing.T) {
	base := solidImage(4, 4, color.NRGBA{R: 128, A: 255})
	overlay := solidImage(4, 4, color.NRGBA{R: 255, G: 60, B: 60, A: 200})

	got := renderHalfBlockOverlay(base, overlay, 4, 0)
	if got == "" {
		t.Fatal("expected non-empty output")
	}
	if !strings.Contains(got, "▀") {
		t.Error("expected half-block characters")
	}
}

func TestHalfBlockString_RowCount(t *testing.T) {
	// 4x4 image should produce 2 rows (4 pixels / 2 per row).
	img := solidImage(4, 4, color.NRGBA{R: 128, A: 255})
	got := halfBlockString(img, 4, 4)

	lines := strings.Split(got, "\n")
	if len(lines) != 2 {
		t.Errorf("got %d lines, want 2", len(lines))
	}
}
