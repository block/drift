package tui

import (
	"image"
	"image/color"
	"strconv"
	"strings"

	"golang.org/x/image/draw"
)

// scaleForTerminal calculates target dimensions for rendering an image in the
// terminal. targetWidth is the desired width in columns; maxRows limits output
// height (0 = unconstrained). Returns an even height for half-block pairing.
func scaleForTerminal(srcW, srcH, targetWidth, maxRows int) (dstW, dstH int) {
	// Each terminal cell is roughly 2:1 (height:width), and we pack 2 pixels
	// per cell vertically, so effective pixel aspect is 1:1.
	scale := float64(targetWidth) / float64(srcW)
	dstW = targetWidth
	dstH = max(int(float64(srcH)*scale), 1)

	// Constrain to max rows if set. Each row = 2 pixels.
	if maxRows > 0 && dstH > maxRows*2 {
		dstH = maxRows * 2
		dstW = max(int(float64(srcW)*float64(dstH)/float64(srcH)), 1)
	}

	// Ensure even height for half-block pairing.
	if dstH%2 != 0 {
		dstH++
	}
	return dstW, dstH
}

// scaleImage scales an image to the given terminal dimensions using bilinear
// interpolation.
func scaleImage(img image.Image, dstW, dstH int) *image.NRGBA {
	scaled := image.NewNRGBA(image.Rect(0, 0, dstW, dstH))
	draw.BiLinear.Scale(scaled, scaled.Bounds(), img, img.Bounds(), draw.Over, nil)
	return scaled
}

// renderHalfBlock renders an image as a string using half-block characters (▀)
// with truecolor ANSI escape codes. Each character cell encodes two vertical
// pixels: the top pixel as foreground color and the bottom pixel as background.
//
// targetWidth is the desired width in terminal columns. maxRows limits the
// output height in terminal rows (0 = unconstrained). The image is scaled
// proportionally to fit within both constraints.
func renderHalfBlock(img image.Image, targetWidth, maxRows int) string {
	if img == nil || targetWidth <= 0 {
		return ""
	}

	bounds := img.Bounds()
	srcW, srcH := bounds.Dx(), bounds.Dy()
	if srcW == 0 || srcH == 0 {
		return ""
	}

	dstW, dstH := scaleForTerminal(srcW, srcH, targetWidth, maxRows)
	scaled := scaleImage(img, dstW, dstH)
	return halfBlockString(scaled, dstW, dstH)
}

// renderHalfBlockOverlay renders the base image with a semi-transparent overlay
// composited on top. Used for diff mask visualization.
func renderHalfBlockOverlay(base, overlay image.Image, targetWidth, maxRows int) string {
	if base == nil || targetWidth <= 0 {
		return ""
	}

	bounds := base.Bounds()
	srcW, srcH := bounds.Dx(), bounds.Dy()
	if srcW == 0 || srcH == 0 {
		return ""
	}

	dstW, dstH := scaleForTerminal(srcW, srcH, targetWidth, maxRows)
	dst := scaleImage(base, dstW, dstH)

	if overlay != nil {
		overlayScaled := scaleImage(overlay, dstW, dstH)
		draw.Draw(dst, dst.Bounds(), overlayScaled, image.Point{}, draw.Over)
	}

	return halfBlockString(dst, dstW, dstH)
}

// ANSI truecolor escape sequence prefixes.
const (
	ansiReset = "\x1b[0m"
	ansiFg    = "\x1b[38;2;" // truecolor foreground: \x1b[38;2;R;G;Bm
	ansiBg    = "\x1b[48;2;" // truecolor background: \x1b[48;2;R;G;Bm
)

// halfBlockString converts an NRGBA image to a half-block encoded string.
// Each cell produces: fg(top) + bg(bottom) + "▀", encoding two vertical pixels.
func halfBlockString(img *image.NRGBA, w, h int) string {
	const bytesPerCell = 28 // two SGR sequences (~12 bytes each) + "▀" (3 bytes) + overhead

	var b strings.Builder
	b.Grow(w * (h / 2) * bytesPerCell)

	var buf [20]byte // scratch for strconv.AppendUint

	// writeColor appends "R;G;Bm" for a truecolor SGR parameter.
	writeColor := func(c color.NRGBA) {
		b.Write(strconv.AppendUint(buf[:0], uint64(c.R), 10))
		b.WriteByte(';')
		b.Write(strconv.AppendUint(buf[:0], uint64(c.G), 10))
		b.WriteByte(';')
		b.Write(strconv.AppendUint(buf[:0], uint64(c.B), 10))
		b.WriteByte('m')
	}

	for y := 0; y < h; y += 2 {
		// Reset at start of each line to prevent color bleed from
		// viewport scrolling or line-clipping artifacts.
		b.WriteString(ansiReset)

		for x := 0; x < w; x++ {
			top := img.NRGBAAt(x, y)
			var bot color.NRGBA
			if y+1 < h {
				bot = img.NRGBAAt(x, y+1)
			}

			b.WriteString(ansiFg)
			writeColor(top)
			b.WriteString(ansiBg)
			writeColor(bot)
			b.WriteString("▀")
		}
		b.WriteString(ansiReset)
		if y+2 < h {
			b.WriteByte('\n')
		}
	}

	return b.String()
}
