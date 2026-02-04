package publisher

import "context"

// Publisher отвечает за публикацию файлов домена в целевое хранилище.
type Publisher interface {
	// Publish публикует файлы домена.
	Publish(ctx context.Context, domainID string, files map[string][]byte) error
	// Unpublish удаляет публикацию домена.
	Unpublish(ctx context.Context, domainID string) error
	// GetPublishedPath возвращает путь публикации для домена.
	GetPublishedPath(domainID string) string
}
