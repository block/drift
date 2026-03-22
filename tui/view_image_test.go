package tui

import (
	"fmt"
	"image/color"
	"strings"
	"testing"

	"github.com/block/drift/compare"
)

func testField(label, value string) string {
	return fmt.Sprintf("%s: %s", label, value)
}

func TestFieldAB_BothSame(t *testing.T) {
	got := fieldAB("Format", "png", "png", " → ", testField)
	if got != "Format: png" {
		t.Errorf("got %q, want %q", got, "Format: png")
	}
}

func TestFieldAB_BothDiffer(t *testing.T) {
	got := fieldAB("Format", "png", "jpeg", " → ", testField)
	want := "Format: png → jpeg"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestFieldAB_OnlyA(t *testing.T) {
	got := fieldAB("Size", "128x128", "", " → ", testField)
	if got != "Size: 128x128" {
		t.Errorf("got %q, want %q", got, "Size: 128x128")
	}
}

func TestFieldAB_OnlyB(t *testing.T) {
	got := fieldAB("Size", "", "64x64", " → ", testField)
	if got != "Size: 64x64" {
		t.Errorf("got %q, want %q", got, "Size: 64x64")
	}
}

func TestFieldAB_BothEmpty(t *testing.T) {
	got := fieldAB("Color", "", "", " → ", testField)
	if got != "" {
		t.Errorf("got %q, want empty", got)
	}
}

func TestFieldAB_CustomSeparator(t *testing.T) {
	got := fieldAB("Mode", "old", "new", " -> ", testField)
	want := "Mode: old -> new"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestImageViewMode_String(t *testing.T) {
	tests := []struct {
		mode ImageViewMode
		want string
	}{
		{ImageViewSideBySide, "Side by Side"},
		{ImageViewBefore, "Before"},
		{ImageViewAfter, "After"},
		{ImageViewDiff, "Diff"},
		{ImageViewMode(99), "unknown"},
	}
	for _, tt := range tests {
		got := tt.mode.String()
		if got != tt.want {
			t.Errorf("ImageViewMode(%d).String() = %q, want %q", tt.mode, got, tt.want)
		}
	}
}

func TestImageViewModeAvailable(t *testing.T) {
	both := &compare.ImageDiff{
		ImageA:        solidImage(1, 1, color.NRGBA{A: 255}),
		ImageB:        solidImage(1, 1, color.NRGBA{A: 255}),
		DiffMask:      solidImage(1, 1, color.NRGBA{R: 255, A: 200}),
		PixelsChanged: 1,
	}
	addedOnly := &compare.ImageDiff{
		ImageB: solidImage(1, 1, color.NRGBA{A: 255}),
	}

	if !imageViewModeAvailable(ImageViewSideBySide, both) {
		t.Error("SideBySide should be available when both images present")
	}
	if imageViewModeAvailable(ImageViewSideBySide, addedOnly) {
		t.Error("SideBySide should not be available with only one image")
	}
	if !imageViewModeAvailable(ImageViewAfter, addedOnly) {
		t.Error("After should be available when ImageB present")
	}
	if imageViewModeAvailable(ImageViewBefore, addedOnly) {
		t.Error("Before should not be available without ImageA")
	}
	if !imageViewModeAvailable(ImageViewDiff, both) {
		t.Error("Diff should be available when mask and changes present")
	}
	noDiff := &compare.ImageDiff{
		ImageA: solidImage(1, 1, color.NRGBA{A: 255}),
		ImageB: solidImage(1, 1, color.NRGBA{A: 255}),
	}
	if imageViewModeAvailable(ImageViewDiff, noDiff) {
		t.Error("Diff should not be available without diff mask")
	}
}

func TestImageDiffView_CopyableText_NoANSI(t *testing.T) {
	// Verify CopyableText produces clean plain text without ANSI escape codes.
	diff := &compare.ImageDiff{
		FormatA: "png", FormatB: "jpeg",
		WidthA: 100, HeightA: 100,
		WidthB: 200, HeightB: 200,
		ImageA: solidImage(1, 1, color.NRGBA{A: 255}),
		ImageB: solidImage(1, 1, color.NRGBA{A: 255}),
	}
	v := ImageDiffView{Diff: diff, Width: 80}
	got := v.CopyableText()

	if strings.Contains(got, "\x1b") {
		t.Errorf("CopyableText contains ANSI escape codes:\n%s", got)
	}
	if !strings.Contains(got, "png") || !strings.Contains(got, "jpeg") {
		t.Errorf("CopyableText missing format info:\n%s", got)
	}
}
