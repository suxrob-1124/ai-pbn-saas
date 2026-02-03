//go:build cgo

package pipeline

import (
	"bytes"
	"fmt"
	"image"
	_ "image/jpeg"
	_ "image/png"

	"github.com/chai2010/webp"
)

// convertToWebP выполняет перекодирование картинки в webp (использует CGO)
func convertToWebP(input []byte) ([]byte, error) {
	img, _, err := image.Decode(bytes.NewReader(input))
	if err != nil {
		return nil, fmt.Errorf("decode failed: %w", err)
	}
	var buf bytes.Buffer
	if err := webp.Encode(&buf, img, &webp.Options{Lossless: false, Quality: 85}); err != nil {
		return nil, fmt.Errorf("encode failed: %w", err)
	}
	return buf.Bytes(), nil
}
