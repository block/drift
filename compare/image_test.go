package compare

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"testing"
)

const testImageDir = "testdata/images"

// writePNG creates a PNG file from an image.
func writePNG(t *testing.T, path string, img image.Image) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if err := png.Encode(f, img); err != nil {
		t.Fatal(err)
	}
}

// copyFixture copies a testdata image into a temp directory under the given name.
func copyFixture(t *testing.T, fixture, destDir, name string) {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(testImageDir, fixture))
	if err != nil {
		t.Fatalf("reading fixture %s: %v", fixture, err)
	}
	dest := filepath.Join(destDir, name)
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(dest, data, 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestCompareImage_Identical(t *testing.T) {
	dirA := t.TempDir()
	dirB := t.TempDir()

	copyFixture(t, "a.png", dirA, "icon.png")
	copyFixture(t, "a.png", dirB, "icon.png")

	diff, err := compareImage(dirA, dirB, "icon.png", Modified)
	if err != nil {
		t.Fatalf("compareImage: %v", err)
	}

	if diff.PixelsChanged != 0 {
		t.Errorf("pixels changed = %d, want 0", diff.PixelsChanged)
	}
	if diff.WidthA != 128 || diff.HeightA != 128 {
		t.Errorf("dimensions A = %dx%d, want 128x128", diff.WidthA, diff.HeightA)
	}
	if diff.FormatA != "png" {
		t.Errorf("format A = %q, want png", diff.FormatA)
	}
}

func TestCompareImage_Modified(t *testing.T) {
	dirA := t.TempDir()
	dirB := t.TempDir()

	// Landscape A vs B: sun shifts position and color.
	copyFixture(t, "a.png", dirA, "scene.png")
	copyFixture(t, "b.png", dirB, "scene.png")

	diff, err := compareImage(dirA, dirB, "scene.png", Modified)
	if err != nil {
		t.Fatalf("compareImage: %v", err)
	}

	if diff.PixelsChanged == 0 {
		t.Error("expected pixels to differ between modified scenes")
	}
	if diff.PixelsTotal != 128*128 {
		t.Errorf("pixels total = %d, want %d", diff.PixelsTotal, 128*128)
	}
	if diff.ChangePercent <= 0 {
		t.Error("expected non-zero change percentage")
	}
	if diff.DiffMask == nil {
		t.Error("expected diff mask to be non-nil")
	}
	if diff.ChangeBounds.Empty() {
		t.Error("expected non-empty change bounds")
	}
}

func TestCompareImage_DimensionsDiffer(t *testing.T) {
	dirA := t.TempDir()
	dirB := t.TempDir()

	copyFixture(t, "a.png", dirA, "icon.png")

	// Create a cropped 64x64 version for B.
	srcData, err := os.ReadFile(filepath.Join(testImageDir, "a.png"))
	if err != nil {
		t.Fatal(err)
	}
	src, err := png.Decode(bytes.NewReader(srcData))
	if err != nil {
		t.Fatal(err)
	}
	// Crop to 64x64 from the original 128x128.
	cropped := src.(interface {
		SubImage(r image.Rectangle) image.Image
	}).SubImage(image.Rect(0, 0, 64, 64))
	writePNG(t, filepath.Join(dirB, "icon.png"), cropped)

	diff, err := compareImage(dirA, dirB, "icon.png", Modified)
	if err != nil {
		t.Fatalf("compareImage: %v", err)
	}

	if diff.PixelsTotal != 0 {
		t.Errorf("pixels total = %d, want 0 (dims differ)", diff.PixelsTotal)
	}
	if diff.WidthA != 128 || diff.WidthB != 64 {
		t.Errorf("widths = %d,%d, want 128,64", diff.WidthA, diff.WidthB)
	}
}

func TestCompareImage_Added(t *testing.T) {
	dirB := t.TempDir()

	copyFixture(t, "b.png", dirB, "new.png")

	diff, err := compareImage("", dirB, "new.png", Added)
	if err != nil {
		t.Fatalf("compareImage: %v", err)
	}

	if diff.ImageA != nil {
		t.Error("expected ImageA to be nil for added file")
	}
	if diff.ImageB == nil {
		t.Error("expected ImageB to be non-nil")
	}
	if diff.WidthB != 128 || diff.HeightB != 128 {
		t.Errorf("dimensions B = %dx%d, want 128x128", diff.WidthB, diff.HeightB)
	}
}

func TestCompareImage_Removed(t *testing.T) {
	dirA := t.TempDir()

	copyFixture(t, "a.png", dirA, "old.png")

	diff, err := compareImage(dirA, "", "old.png", Removed)
	if err != nil {
		t.Fatalf("compareImage: %v", err)
	}

	if diff.ImageA == nil {
		t.Error("expected ImageA to be non-nil")
	}
	if diff.ImageB != nil {
		t.Error("expected ImageB to be nil for removed file")
	}
}

func TestClassifyPath_Image(t *testing.T) {
	for _, ext := range []string{".png", ".jpg", ".jpeg", ".gif", ".tiff", ".bmp", ".webp"} {
		kind := classifyPath("assets/icon"+ext, false)
		if kind != KindImage {
			t.Errorf("classifyPath(%q) = %v, want KindImage", ext, kind)
		}
	}
}

func TestDetectMode_Image(t *testing.T) {
	dir := t.TempDir()

	copyFixture(t, "a.png", dir, "a.png")
	copyFixture(t, "b.png", dir, "b.png")

	mode, err := detectMode(filepath.Join(dir, "a.png"), filepath.Join(dir, "b.png"))
	if err != nil {
		t.Fatalf("detectMode: %v", err)
	}
	if mode != ModeImage {
		t.Errorf("mode = %q, want image", mode)
	}
}

func TestDetailImage_Standalone(t *testing.T) {
	dir := t.TempDir()

	copyFixture(t, "a.png", dir, "a.png")
	copyFixture(t, "b.png", dir, "b.png")

	a := filepath.Join(dir, "a.png")
	b := filepath.Join(dir, "b.png")

	result, err := Compare(a, b, "")
	if err != nil {
		t.Fatalf("Compare: %v", err)
	}
	if result.Mode != ModeImage {
		t.Fatalf("mode = %q, want image", result.Mode)
	}

	detail, err := Detail(result, result.Root)
	if err != nil {
		t.Fatalf("Detail: %v", err)
	}
	if detail.Image == nil {
		t.Fatal("expected image diff")
	}
	if detail.Image.PixelsChanged == 0 {
		t.Error("expected non-zero pixel changes")
	}
}

func TestDetailImage_InTree(t *testing.T) {
	dirA := t.TempDir()
	dirB := t.TempDir()

	copyFixture(t, "a.png", dirA, "assets/scene.png")
	copyFixture(t, "b.png", dirB, "assets/scene.png")
	writeFile(t, filepath.Join(dirA, "README.txt"), "version 1\n")
	writeFile(t, filepath.Join(dirB, "README.txt"), "version 2\n")

	result, err := Compare(dirA, dirB, "tree")
	if err != nil {
		t.Fatalf("Compare: %v", err)
	}

	node := findNodeInTree(result.Root, "scene.png")
	if node == nil {
		t.Fatal("scene.png not found in tree")
	}
	if node.Kind != KindImage {
		t.Fatalf("kind = %v, want image", node.Kind)
	}

	detail, err := Detail(result, node)
	if err != nil {
		t.Fatalf("Detail: %v", err)
	}
	if detail.Image == nil {
		t.Fatal("expected image diff")
	}
	if detail.Image.PixelsChanged == 0 {
		t.Error("expected non-zero pixel changes")
	}
}

func TestComputePixelDiff_NRGBAFastPath(t *testing.T) {
	a := image.NewNRGBA(image.Rect(0, 0, 4, 4))
	b := image.NewNRGBA(image.Rect(0, 0, 4, 4))

	// Fill both with the same color.
	for i := range a.Pix {
		a.Pix[i] = 128
		b.Pix[i] = 128
	}
	// Change one pixel in b.
	b.SetNRGBA(2, 2, color.NRGBA{R: 255, G: 0, B: 0, A: 255})

	d := &ImageDiff{ImageA: a, ImageB: b}
	d.computePixelDiff()

	if d.PixelsChanged != 1 {
		t.Errorf("pixels changed = %d, want 1", d.PixelsChanged)
	}
	if d.PixelsTotal != 16 {
		t.Errorf("pixels total = %d, want 16", d.PixelsTotal)
	}
	if d.ChangeBounds != image.Rect(2, 2, 3, 3) {
		t.Errorf("change bounds = %v, want (2,2)-(3,3)", d.ChangeBounds)
	}
}

func TestComputePixelDiff_Identical(t *testing.T) {
	a := image.NewNRGBA(image.Rect(0, 0, 2, 2))
	b := image.NewNRGBA(image.Rect(0, 0, 2, 2))

	d := &ImageDiff{ImageA: a, ImageB: b}
	d.computePixelDiff()

	if d.PixelsChanged != 0 {
		t.Errorf("pixels changed = %d, want 0", d.PixelsChanged)
	}
	if d.ChangePercent != 0 {
		t.Errorf("change percent = %f, want 0", d.ChangePercent)
	}
}

func TestComputePixelDiff_GenericFallback(t *testing.T) {
	// Use image.RGBA (not NRGBA) to exercise the generic fallback path.
	a := image.NewRGBA(image.Rect(0, 0, 3, 3))
	b := image.NewRGBA(image.Rect(0, 0, 3, 3))

	// Change two pixels.
	b.SetRGBA(0, 0, color.RGBA{R: 255, G: 0, B: 0, A: 255})
	b.SetRGBA(1, 1, color.RGBA{R: 0, G: 255, B: 0, A: 255})

	d := &ImageDiff{ImageA: a, ImageB: b}
	d.computePixelDiff()

	if d.PixelsChanged != 2 {
		t.Errorf("pixels changed = %d, want 2", d.PixelsChanged)
	}
	if d.ChangeBounds != image.Rect(0, 0, 2, 2) {
		t.Errorf("change bounds = %v, want (0,0)-(2,2)", d.ChangeBounds)
	}
}

func TestColorModelName(t *testing.T) {
	tests := []struct {
		model color.Model
		want  string
	}{
		{color.RGBAModel, "RGBA"},
		{color.NRGBAModel, "NRGBA"},
		{color.GrayModel, "Gray"},
		{color.YCbCrModel, "YCbCr"},
		{color.CMYKModel, "CMYK"},
		{nil, "unknown"},
	}
	for _, tt := range tests {
		got := colorModelName(tt.model)
		if got != tt.want {
			t.Errorf("colorModelName(%v) = %q, want %q", tt.model, got, tt.want)
		}
	}
}

func TestIsImageExt(t *testing.T) {
	for _, ext := range []string{".png", ".jpg", ".jpeg", ".gif", ".tiff", ".tif", ".bmp", ".webp"} {
		if !isImageExt(ext) {
			t.Errorf("isImageExt(%q) = false, want true", ext)
		}
	}
	for _, ext := range []string{".pdf", ".txt", ".car", ".svg", ""} {
		if isImageExt(ext) {
			t.Errorf("isImageExt(%q) = true, want false", ext)
		}
	}
}
