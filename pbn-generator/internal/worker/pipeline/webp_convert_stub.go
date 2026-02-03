//go:build !cgo

package pipeline

import "fmt"

// Fallback: при сборке без CGO возвращаем ошибку, чтобы можно было сохранить оригинал
func convertToWebP(input []byte) ([]byte, error) {
	return nil, fmt.Errorf("webp conversion not available without cgo")
}
