package linkbuilder

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"golang.org/x/net/html"
)

// LinkBuilder описывает операции линкбилдинга.
type LinkBuilder interface {
	ProcessTask(ctx context.Context, taskID string) error
	FindAnchor(htmlContent, anchorText string) (position int, found bool)
	InsertLink(htmlContent string, position int, anchorText, targetURL string) string
	GenerateContent(ctx context.Context, anchorText, targetURL, pageContext string) (string, error)
}

// Task описывает задачу линкбилдинга.
type Task struct {
	ID               string
	DomainID         string
	AnchorText       string
	TargetURL        string
	Status           string
	FoundLocation    string
	GeneratedContent string
	ErrorMessage     string
	Attempts         int
}

// TaskUpdates описывает изменения задачи линкбилдинга.
type TaskUpdates struct {
	Status           *string
	FoundLocation    *string
	GeneratedContent *string
	ErrorMessage     *string
	Attempts         *int
}

// TaskStore определяет операции над задачами линкбилдинга.
type TaskStore interface {
	Get(ctx context.Context, taskID string) (Task, error)
	Update(ctx context.Context, taskID string, updates TaskUpdates) error
}

// HTMLStore определяет операции чтения/записи HTML контента.
type HTMLStore interface {
	Load(ctx context.Context, domainID string) (string, error)
	Save(ctx context.Context, domainID string, htmlContent string) error
}

// ContentGenerator генерирует контент, если якорь не найден.
type ContentGenerator interface {
	Generate(ctx context.Context, anchorText, targetURL, pageContext string) (string, error)
}

// Builder реализует LinkBuilder.
type Builder struct {
	Tasks     TaskStore
	HTML      HTMLStore
	Generator ContentGenerator
	FileName  string
}

// NewBuilder создает новый Builder.
func NewBuilder(tasks TaskStore, html HTMLStore, generator ContentGenerator) *Builder {
	return &Builder{Tasks: tasks, HTML: html, Generator: generator, FileName: "index.html"}
}

// ProcessTask выполняет задачу линкбилдинга.
func (b *Builder) ProcessTask(ctx context.Context, taskID string) error {
	if b == nil {
		return errors.New("linkbuilder: nil builder")
	}
	if b.Tasks == nil {
		return errors.New("linkbuilder: task store is nil")
	}
	if b.HTML == nil {
		return errors.New("linkbuilder: html store is nil")
	}
	if strings.TrimSpace(taskID) == "" {
		return errors.New("linkbuilder: task id is required")
	}

	task, err := b.Tasks.Get(ctx, taskID)
	if err != nil {
		return fmt.Errorf("linkbuilder: get task: %w", err)
	}

	attempts := task.Attempts + 1

	htmlContent, err := b.HTML.Load(ctx, task.DomainID)
	if err != nil {
		updateErr := b.Tasks.Update(ctx, task.ID, TaskUpdates{
			Status:       strPtr("failed"),
			ErrorMessage: strPtr(fmt.Sprintf("load html failed: %v", err)),
			Attempts:     &attempts,
		})
		if updateErr != nil {
			return fmt.Errorf("linkbuilder: update task after load error: %w", updateErr)
		}
		return fmt.Errorf("linkbuilder: load html: %w", err)
	}

	pos, found := b.FindAnchor(htmlContent, task.AnchorText)
	if found {
		updated := b.InsertLink(htmlContent, pos, task.AnchorText, task.TargetURL)
		if updated == htmlContent {
			updateErr := b.Tasks.Update(ctx, task.ID, TaskUpdates{
				Status:       strPtr("failed"),
				ErrorMessage: strPtr("failed to insert link"),
				Attempts:     &attempts,
			})
			if updateErr != nil {
				return fmt.Errorf("linkbuilder: update task after insert error: %w", updateErr)
			}
			return fmt.Errorf("linkbuilder: failed to insert link")
		}
		if err := b.HTML.Save(ctx, task.DomainID, updated); err != nil {
			updateErr := b.Tasks.Update(ctx, task.ID, TaskUpdates{
				Status:       strPtr("failed"),
				ErrorMessage: strPtr(fmt.Sprintf("save html failed: %v", err)),
				Attempts:     &attempts,
			})
			if updateErr != nil {
				return fmt.Errorf("linkbuilder: update task after save error: %w", updateErr)
			}
			return fmt.Errorf("linkbuilder: save html: %w", err)
		}
		foundLocation := fmt.Sprintf("%s:%d", b.fileName(), pos)
		if err := b.Tasks.Update(ctx, task.ID, TaskUpdates{
			Status:        strPtr("inserted"),
			FoundLocation: &foundLocation,
			Attempts:      &attempts,
		}); err != nil {
			return fmt.Errorf("linkbuilder: update task after insert: %w", err)
		}
		return nil
	}

	generated, err := b.GenerateContent(ctx, task.AnchorText, task.TargetURL, htmlContent)
	if err != nil {
		updateErr := b.Tasks.Update(ctx, task.ID, TaskUpdates{
			Status:       strPtr("failed"),
			ErrorMessage: strPtr(fmt.Sprintf("generate content failed: %v", err)),
			Attempts:     &attempts,
		})
		if updateErr != nil {
			return fmt.Errorf("linkbuilder: update task after generate error: %w", updateErr)
		}
		return fmt.Errorf("linkbuilder: generate content: %w", err)
	}

	updated := appendContent(htmlContent, generated)
	if err := b.HTML.Save(ctx, task.DomainID, updated); err != nil {
		updateErr := b.Tasks.Update(ctx, task.ID, TaskUpdates{
			Status:       strPtr("failed"),
			ErrorMessage: strPtr(fmt.Sprintf("save html failed: %v", err)),
			Attempts:     &attempts,
		})
		if updateErr != nil {
			return fmt.Errorf("linkbuilder: update task after save error: %w", updateErr)
		}
		return fmt.Errorf("linkbuilder: save html: %w", err)
	}

	if err := b.Tasks.Update(ctx, task.ID, TaskUpdates{
		Status:           strPtr("generated"),
		GeneratedContent: &generated,
		Attempts:         &attempts,
	}); err != nil {
		return fmt.Errorf("linkbuilder: update task after generate: %w", err)
	}

	return nil
}

// FindAnchor ищет якорный текст в HTML, исключая заголовки и существующие ссылки.
func (b *Builder) FindAnchor(htmlContent, anchorText string) (position int, found bool) {
	return b.FindAnchorInBody(htmlContent, anchorText, false)
}

// FindAnchorInBody ищет якорный текст в пределах <body> и может включать уже существующие ссылки.
func (b *Builder) FindAnchorInBody(htmlContent, anchorText string, allowInAnchor bool) (position int, found bool) {
	anchorText = strings.TrimSpace(anchorText)
	if anchorText == "" {
		return -1, false
	}
	lowerAnchor := strings.ToLower(anchorText)
	tokenizer := html.NewTokenizer(strings.NewReader(htmlContent))
	offset := 0
	inBody := false
	headingDepth := 0
	inScript := false
	inStyle := false
	inAnchor := false

	for {
		tt := tokenizer.Next()
		if tt == html.ErrorToken {
			return -1, false
		}
		raw := tokenizer.Raw()
		base := offset

		switch tt {
		case html.StartTagToken:
			tok := tokenizer.Token()
			tag := strings.ToLower(tok.Data)
			switch tag {
			case "body":
				inBody = true
			case "script":
				inScript = true
			case "style":
				inStyle = true
			case "a":
				inAnchor = true
			case "h1", "h2", "h3", "h4", "h5", "h6":
				headingDepth++
			}
		case html.EndTagToken:
			tok := tokenizer.Token()
			tag := strings.ToLower(tok.Data)
			switch tag {
			case "body":
				inBody = false
			case "script":
				inScript = false
			case "style":
				inStyle = false
			case "a":
				inAnchor = false
			case "h1", "h2", "h3", "h4", "h5", "h6":
				if headingDepth > 0 {
					headingDepth--
				}
			}
		case html.TextToken:
			if inBody && headingDepth == 0 && !inScript && !inStyle && (allowInAnchor || !inAnchor) {
				lowerText := strings.ToLower(string(raw))
				if idx := strings.Index(lowerText, lowerAnchor); idx != -1 {
					return base + idx, true
				}
			}
		}

		offset += len(raw)
	}
}

// InsertLink вставляет ссылку в HTML по позиции.
func (b *Builder) InsertLink(htmlContent string, position int, anchorText, targetURL string) string {
	anchorText = strings.TrimSpace(anchorText)
	if position < 0 || anchorText == "" {
		return htmlContent
	}
	end := position + len(anchorText)
	if end > len(htmlContent) {
		return htmlContent
	}
	src := htmlContent[position:end]
	if !strings.EqualFold(src, anchorText) {
		return htmlContent
	}
	link := fmt.Sprintf("<a href=\"%s\">%s</a>", targetURL, src)
	return htmlContent[:position] + link + htmlContent[end:]
}

// GenerateContent генерирует контент, если якорь не найден.
func (b *Builder) GenerateContent(ctx context.Context, anchorText, targetURL, pageContext string) (string, error) {
	if b.Generator == nil {
		return "", errors.New("linkbuilder: generator is nil")
	}
	return b.Generator.Generate(ctx, anchorText, targetURL, pageContext)
}

func appendContent(htmlContent, addition string) string {
	addition = strings.TrimSpace(addition)
	if addition == "" {
		return htmlContent
	}
	lower := strings.ToLower(htmlContent)
	idx := strings.LastIndex(lower, "</body>")
	if idx == -1 {
		return htmlContent + "\n" + addition
	}
	return htmlContent[:idx] + addition + "\n" + htmlContent[idx:]
}

func (b *Builder) fileName() string {
	if strings.TrimSpace(b.FileName) == "" {
		return "index.html"
	}
	return b.FileName
}

func strPtr(val string) *string {
	return &val
}
