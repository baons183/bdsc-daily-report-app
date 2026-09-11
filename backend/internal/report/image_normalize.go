package report

import (
	"bytes"
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	"image/png"
	"os"

	_ "golang.org/x/image/webp"
)

// preparePDFImage always decodes the source file with Go's own image
// package (png/jpeg/gif plus the blank-imported webp decoder above cover
// everything phones/screenshot tools realistically produce) and re-encodes
// it as PNG in memory before handing it to fpdf.
//
// Originally this only did that for "unrecognized" extensions and passed
// .png/.jpg/.gif straight through — but fpdf's own built-in PNG parser is a
// minimal hand-rolled one, not the full image/png stdlib, and it rejected
// several real-world "valid to every browser" PNGs with "not a PNG buffer".
// Always routing through Go's decoder + a single trusted re-encoder
// sidesteps every such quirk regardless of what created the source file.
func preparePDFImage(path string) (imgType string, data []byte, err error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return "", nil, err
	}
	img, _, err := image.Decode(bytes.NewReader(raw))
	if err != nil {
		return "", nil, fmt.Errorf("dinh dang anh khong ho tro: %w", err)
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return "", nil, err
	}
	return "PNG", buf.Bytes(), nil
}
