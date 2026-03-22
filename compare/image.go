package compare

import (
	"bytes"
	"fmt"
	"image"
	"image/color"

	// Register decoders for image.Decode.
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"

	_ "golang.org/x/image/bmp"
	_ "golang.org/x/image/tiff"
	_ "golang.org/x/image/webp"
)

// ImageDiff holds the result of comparing two images.
type ImageDiff struct {
	WidthA  int `json:"width_a"`
	HeightA int `json:"height_a"`
	WidthB  int `json:"width_b"`
	HeightB int `json:"height_b"`

	FormatA string `json:"format_a"`
	FormatB string `json:"format_b"`

	ColorModelA string `json:"color_model_a"`
	ColorModelB string `json:"color_model_b"`

	// Pixel diff stats (only populated when dimensions match).
	PixelsChanged int     `json:"pixels_changed"`
	PixelsTotal   int     `json:"pixels_total"`
	ChangePercent float64 `json:"change_percent"`

	// Bounding box of changed region.
	ChangeBounds image.Rectangle `json:"change_bounds"`

	// Decoded images for TUI rendering (not serialized).
	ImageA image.Image `json:"-"`
	ImageB image.Image `json:"-"`
	// DiffMask highlights changed pixels (nil when dimensions differ).
	DiffMask image.Image `json:"-"`
}

// compareImage decodes two images and produces a diff.
func compareImage(sourceA, sourceB, relPath string, status DiffStatus) (*ImageDiff, error) {
	diff := &ImageDiff{}

	if status != Added {
		dataA, err := readContent(sourceA, relPath)
		if err != nil {
			return nil, fmt.Errorf("reading image A: %w", err)
		}
		imgA, fmtA, err := image.Decode(bytes.NewReader(dataA))
		if err != nil {
			return nil, fmt.Errorf("decoding image A: %w", err)
		}
		diff.ImageA = imgA
		diff.FormatA = fmtA
		diff.WidthA = imgA.Bounds().Dx()
		diff.HeightA = imgA.Bounds().Dy()
		diff.ColorModelA = colorModelName(imgA.ColorModel())
	}

	if status != Removed {
		dataB, err := readContent(sourceB, relPath)
		if err != nil {
			return nil, fmt.Errorf("reading image B: %w", err)
		}
		imgB, fmtB, err := image.Decode(bytes.NewReader(dataB))
		if err != nil {
			return nil, fmt.Errorf("decoding image B: %w", err)
		}
		diff.ImageB = imgB
		diff.FormatB = fmtB
		diff.WidthB = imgB.Bounds().Dx()
		diff.HeightB = imgB.Bounds().Dy()
		diff.ColorModelB = colorModelName(imgB.ColorModel())
	}

	// Compute pixel diff when both images exist and dimensions match.
	if diff.ImageA != nil && diff.ImageB != nil &&
		diff.WidthA == diff.WidthB && diff.HeightA == diff.HeightB {
		diff.computePixelDiff()
	}

	return diff, nil
}

// computePixelDiff compares images pixel-by-pixel and builds a diff mask.
func (d *ImageDiff) computePixelDiff() {
	bounds := d.ImageA.Bounds()
	w, h := bounds.Dx(), bounds.Dy()
	d.PixelsTotal = w * h

	mask := image.NewNRGBA(bounds)
	minX, minY := w, h
	maxX, maxY := 0, 0

	diffPixel := color.NRGBA{R: 255, G: 60, B: 60, A: 200}

	// Fast path: direct pixel slice access when both images are NRGBA.
	nrgbaA, okA := d.ImageA.(*image.NRGBA)
	nrgbaB, okB := d.ImageB.(*image.NRGBA)

	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			var differs bool
			if okA && okB {
				iA := nrgbaA.PixOffset(x, y)
				iB := nrgbaB.PixOffset(x, y)
				differs = nrgbaA.Pix[iA] != nrgbaB.Pix[iB] ||
					nrgbaA.Pix[iA+1] != nrgbaB.Pix[iB+1] ||
					nrgbaA.Pix[iA+2] != nrgbaB.Pix[iB+2] ||
					nrgbaA.Pix[iA+3] != nrgbaB.Pix[iB+3]
			} else {
				rA, gA, bA, aA := d.ImageA.At(x, y).RGBA()
				rB, gB, bB, aB := d.ImageB.At(x, y).RGBA()
				differs = rA != rB || gA != gB || bA != bB || aA != aB
			}

			if differs {
				d.PixelsChanged++
				mask.SetNRGBA(x, y, diffPixel)

				if x < minX {
					minX = x
				}
				if x > maxX {
					maxX = x
				}
				if y < minY {
					minY = y
				}
				if y > maxY {
					maxY = y
				}
			}
		}
	}

	if d.PixelsChanged > 0 {
		d.ChangePercent = float64(d.PixelsChanged) / float64(d.PixelsTotal) * 100
		d.ChangeBounds = image.Rect(minX, minY, maxX+1, maxY+1)
	}
	d.DiffMask = mask
}

// colorModelName returns a human-readable name for an image color model.
func colorModelName(m color.Model) string {
	switch m {
	case color.RGBAModel:
		return "RGBA"
	case color.RGBA64Model:
		return "RGBA64"
	case color.NRGBAModel:
		return "NRGBA"
	case color.NRGBA64Model:
		return "NRGBA64"
	case color.AlphaModel:
		return "Alpha"
	case color.Alpha16Model:
		return "Alpha16"
	case color.GrayModel:
		return "Gray"
	case color.Gray16Model:
		return "Gray16"
	case color.CMYKModel:
		return "CMYK"
	case color.YCbCrModel:
		return "YCbCr"
	default:
		return "unknown"
	}
}
