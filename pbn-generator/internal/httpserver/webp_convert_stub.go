//go:build !cgo

package httpserver

import "fmt"

func convertToWebP(_ []byte) ([]byte, error) {
	return nil, fmt.Errorf("webp conversion not available without cgo")
}
