package report

import (
	"bytes"
	"fmt"
	"image"
	_ "image/gif"
	"image/jpeg"
	_ "image/png"
	"os"

	"golang.org/x/image/draw"
	_ "golang.org/x/image/webp"
)

// maxPhotoDim caps the longest side of any embedded photo, in pixels. Phone
// cameras routinely shoot 4000px+ across, far beyond anything a printed PDF
// photo (a few cm on the page) can show — encoding at full resolution only
// bloats the file and slows down emailing it.
const maxPhotoDim = 1600

// preparePDFImage always decodes the source file with Go's own image
// package (png/jpeg/gif plus the blank-imported webp decoder above cover
// everything phones/screenshot tools realistically produce), downsizes it
// if needed, and re-encodes it as JPEG in memory before handing it to fpdf.
//
// Originally this only did that for "unrecognized" extensions and passed
// .png/.jpg/.gif straight through — but fpdf's own built-in PNG parser is a
// minimal hand-rolled one, not the full image/png stdlib, and it rejected
// several real-world "valid to every browser" PNGs with "not a PNG buffer".
// Always routing through Go's decoder + a single trusted re-encoder
// sidesteps every such quirk regardless of what created the source file.
//
// Re-encoding as JPEG (not PNG) matters just as much: these are photos from
// site cameras, and PNG's lossless compression made real photos balloon to
// 10-20x their original size (one report hit ~19MB and its email upload
// started timing out). JPEG at quality 85 is visually fine for print and a
// small fraction of the size.
func preparePDFImage(path string) (imgType string, data []byte, err error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return "", nil, err
	}
	img, _, err := image.Decode(bytes.NewReader(raw))
	if err != nil {
		return "", nil, fmt.Errorf("dinh dang anh khong ho tro: %w", err)
	}
	img = downscale(img, maxPhotoDim)
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: 85}); err != nil {
		return "", nil, err
	}
	return "JPEG", buf.Bytes(), nil
}

// downscale shrinks img so its longest side is at most maxDim, preserving
// aspect ratio. Returns img unchanged if it's already within bounds.
func downscale(img image.Image, maxDim int) image.Image {
	b := img.Bounds()
	w, h := b.Dx(), b.Dy()
	if w <= maxDim && h <= maxDim {
		return img
	}
	scale := float64(maxDim) / float64(w)
	if float64(maxDim)/float64(h) < scale {
		scale = float64(maxDim) / float64(h)
	}
	nw, nh := int(float64(w)*scale), int(float64(h)*scale)
	dst := image.NewRGBA(image.Rect(0, 0, nw, nh))
	draw.CatmullRom.Scale(dst, dst.Bounds(), img, b, draw.Over, nil)
	return dst
}
