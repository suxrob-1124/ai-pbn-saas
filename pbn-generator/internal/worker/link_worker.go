package worker

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"

	"obzornik-pbn-generator/internal/config"
	"obzornik-pbn-generator/internal/crypto/secretbox"
	"obzornik-pbn-generator/internal/linkbuilder"
	"obzornik-pbn-generator/internal/llm"
	"obzornik-pbn-generator/internal/retry"
	"obzornik-pbn-generator/internal/store/sqlstore"
)

// LinkTaskStore описывает операции над задачами линкбилдинга.
type LinkTaskStore interface {
	Get(ctx context.Context, taskID string) (*sqlstore.LinkTask, error)
	Update(ctx context.Context, taskID string, updates sqlstore.LinkTaskUpdates) error
}

// DomainStore описывает операции над доменами.
type DomainStore interface {
	Get(ctx context.Context, id string) (sqlstore.Domain, error)
	UpdateLinkStatus(ctx context.Context, id string, status string) error
	UpdateLinkState(ctx context.Context, id string, status string, lastTaskID string, filePath string, anchorSnapshot string) error
}

// ProjectStore описывает операции над проектами.
type ProjectStore interface {
	GetByID(ctx context.Context, id string) (sqlstore.Project, error)
}

// UserStore описывает операции над пользователями.
type UserStore interface {
	GetAPIKey(ctx context.Context, email string) ([]byte, *time.Time, error)
}

// SiteFileStore описывает операции с файлами домена.
type SiteFileStore interface {
	GetByPath(ctx context.Context, domainID, path string) (*sqlstore.SiteFile, error)
	Create(ctx context.Context, file sqlstore.SiteFile) error
	Update(ctx context.Context, fileID string, content []byte) error
}

// FileEditStore описывает операции с логами изменений файлов.
type FileEditStore interface {
	Create(ctx context.Context, edit sqlstore.FileEdit) error
}

type LLMUsageStore interface {
	CreateEvent(ctx context.Context, item sqlstore.LLMUsageEvent) error
}

type ModelPricingStore interface {
	GetActiveByModel(ctx context.Context, provider, model string, at time.Time) (*sqlstore.LLMModelPricing, error)
}

// ContentGenerator генерирует HTML контент для ссылки.
type ContentGenerator interface {
	Generate(ctx context.Context, anchorText, targetURL, pageContext string) (string, error)
}

// LinkWorker обрабатывает задачи линкбилдинга.
type LinkWorker struct {
	BaseDir   string
	Config    config.Config
	Logger    *zap.SugaredLogger
	Tasks     LinkTaskStore
	Domains   DomainStore
	Projects  ProjectStore
	Users     UserStore
	SiteFiles SiteFileStore
	FileEdits FileEditStore
	LLMUsage  LLMUsageStore
	Pricing   ModelPricingStore
	Generator ContentGenerator
	Now       func() time.Time
}

var (
	errLinkSettingsNotConfigured = errors.New("link settings not configured")
	errAnchorTextRequired        = errors.New("anchor text is required")
	errTargetURLRequired         = errors.New("target url is required")
	errNoHTMLFiles               = errors.New("no html files found")
	errRelinkSourceNotFound      = errors.New("relink source not found")
	errUnsupportedLinkAction     = errors.New("unsupported link task action")
)

// ProcessTask выполняет задачу линкбилдинга.
func (w *LinkWorker) ProcessTask(ctx context.Context, taskID string) error {
	if w == nil {
		return errors.New("link worker is nil")
	}
	if w.Tasks == nil || w.Domains == nil {
		return errors.New("link worker stores are not configured")
	}
	if strings.TrimSpace(taskID) == "" {
		return errors.New("task id is required")
	}
	if w.Now == nil {
		w.Now = func() time.Time { return time.Now().UTC() }
	}
	logLines := []string{}

	task, err := w.Tasks.Get(ctx, taskID)
	if err != nil {
		return fmt.Errorf("get link task: %w", err)
	}
	attempts := task.Attempts + 1
	action := strings.TrimSpace(strings.ToLower(task.Action))
	if action == "" {
		action = "insert"
	}
	if action != "insert" && action != "remove" {
		return w.failTask(ctx, taskID, attempts, task.CreatedAt, &logLines, fmt.Errorf("%w: %s", errUnsupportedLinkAction, action))
	}
	w.appendLog(ctx, taskID, &logLines, fmt.Sprintf("старт задачи для домена %s", task.DomainID))

	statusSearching := "searching"
	if action == "remove" {
		statusSearching = "removing"
	}
	if err := w.Tasks.Update(ctx, taskID, sqlstore.LinkTaskUpdates{
		Status:   &statusSearching,
		Attempts: &attempts,
	}); err != nil {
		return fmt.Errorf("update task searching: %w", err)
	}
	w.appendLog(ctx, taskID, &logLines, fmt.Sprintf("статус: %s", statusSearching))

	domain, err := w.Domains.Get(ctx, task.DomainID)
	if err != nil {
		return w.failTask(ctx, taskID, attempts, task.CreatedAt, &logLines, fmt.Errorf("domain not found: %w", err))
	}
	if err := w.Domains.UpdateLinkStatus(ctx, domain.ID, statusSearching); err != nil {
		return w.failTask(ctx, taskID, attempts, task.CreatedAt, &logLines, fmt.Errorf("update domain link status: %w", err))
	}
	baseDir, err := w.ensureBaseDir()
	if err != nil {
		return w.failTask(ctx, taskID, attempts, task.CreatedAt, &logLines, err)
	}
	domainDir, err := w.domainDir(baseDir, domain.URL)
	if err != nil {
		return w.failTask(ctx, taskID, attempts, task.CreatedAt, &logLines, err)
	}

	htmlFiles, err := listHTMLFiles(domainDir)
	if err != nil {
		return w.failTask(ctx, taskID, attempts, task.CreatedAt, &logLines, err)
	}
	if len(htmlFiles) == 0 {
		return w.failTask(ctx, taskID, attempts, task.CreatedAt, &logLines, errNoHTMLFiles)
	}

	builder := linkbuilder.NewBuilder(nil, nil, w.Generator)
	if action == "remove" {
		return w.processRemoveTask(ctx, task, domain, domainDir, htmlFiles, attempts, &logLines)
	}
	prevTask := w.loadPreviousTask(ctx, task, domain)
	skipDeepReplace := false
	if domain.LinkStatus.Valid && strings.EqualFold(strings.TrimSpace(domain.LinkStatus.String), "needs_relink") {
		skipDeepReplace = true
		w.appendLog(ctx, taskID, &logLines, "сайт перегенерирован: пропускаем глубокий поиск старой ссылки")
	}
	if prevTask != nil {
		if prevTask.FoundLocation.Valid {
			rel := parseFoundLocation(prevTask.FoundLocation.String)
			if rel != "" {
				if replaced, err := w.replaceInFile(ctx, task, prevTask, attempts, domain, domainDir, rel, builder, &logLines); err != nil {
					return w.failTask(ctx, taskID, attempts, task.CreatedAt, &logLines, err)
				} else if replaced {
					return nil
				}
			}
		}
		if !skipDeepReplace {
			if replaced, err := w.replaceAcrossFiles(ctx, task, prevTask, attempts, domain, domainDir, htmlFiles, builder, &logLines); err != nil {
				return w.failTask(ctx, taskID, attempts, task.CreatedAt, &logLines, err)
			} else if replaced {
				return nil
			}
		}
		if found, err := w.completeIfLinkExists(ctx, task, domain, domainDir, htmlFiles, attempts, &logLines); err != nil {
			return w.failTask(ctx, taskID, attempts, task.CreatedAt, &logLines, err)
		} else if found {
			return nil
		}
		return w.failTask(ctx, taskID, attempts, task.CreatedAt, &logLines, fmt.Errorf("%w: domain=%s prev_task=%s", errRelinkSourceNotFound, domain.ID, prevTask.ID))
	}

	if found, err := w.completeIfLinkExists(ctx, task, domain, domainDir, htmlFiles, attempts, &logLines); err != nil {
		return w.failTask(ctx, taskID, attempts, task.CreatedAt, &logLines, err)
	} else if found {
		return nil
	}

	for _, rel := range htmlFiles {
		full := filepath.Join(domainDir, filepath.FromSlash(rel))
		content, err := os.ReadFile(full)
		if err != nil {
			return w.failTask(ctx, taskID, attempts, task.CreatedAt, &logLines, fmt.Errorf("read html failed: %w", err))
		}
		pos, found := builder.FindAnchor(string(content), task.AnchorText)
		if !found {
			continue
		}
		updated := builder.InsertLink(string(content), pos, task.AnchorText, task.TargetURL)
		if updated == string(content) {
			return w.failTask(ctx, taskID, attempts, task.CreatedAt, &logLines, errors.New("failed to insert link"))
		}
		if err := os.WriteFile(full, []byte(updated), 0o644); err != nil {
			return w.failTask(ctx, taskID, attempts, task.CreatedAt, &logLines, fmt.Errorf("save html failed: %w", err))
		}
		if err := w.recordFileEdit(ctx, task, rel, content, []byte(updated), "link_injection", "link task "+task.ID); err != nil {
			return w.failTask(ctx, taskID, attempts, task.CreatedAt, &logLines, fmt.Errorf("record file edit: %w", err))
		}

		foundLocation := fmt.Sprintf("%s:%d", rel, lineNumber(string(content), pos))
		statusInserted := "inserted"
		completedAt := sql.NullTime{Time: w.Now(), Valid: true}
		clearErr := sql.NullString{}
		if err := w.Tasks.Update(ctx, taskID, sqlstore.LinkTaskUpdates{
			Status:        &statusInserted,
			FoundLocation: &sql.NullString{String: foundLocation, Valid: true},
			ErrorMessage:  &clearErr,
			Attempts:      &attempts,
			CompletedAt:   &completedAt,
		}); err != nil {
			return fmt.Errorf("update task inserted: %w", err)
		}
		if err := w.updateDomainLinkState(ctx, domain.ID, statusInserted, task.ID, rel, task.AnchorText); err != nil {
			return w.failTask(ctx, taskID, attempts, task.CreatedAt, &logLines, err)
		}
		w.appendLog(ctx, taskID, &logLines, fmt.Sprintf("вставлена ссылка в %s", rel))
		return nil
	}

	targetRel := selectTargetHTML(htmlFiles)
	full := filepath.Join(domainDir, filepath.FromSlash(targetRel))
	content, err := os.ReadFile(full)
	if err != nil {
		return w.failTask(ctx, taskID, attempts, task.CreatedAt, &logLines, fmt.Errorf("read html failed: %w", err))
	}

	generated, err := w.generateContent(ctx, task, domain, string(content))
	if err != nil {
		return w.failTask(ctx, taskID, attempts, task.CreatedAt, &logLines, err)
	}
	updated := appendContent(string(content), generated)
	if err := os.WriteFile(full, []byte(updated), 0o644); err != nil {
		return w.failTask(ctx, taskID, attempts, task.CreatedAt, &logLines, fmt.Errorf("save html failed: %w", err))
	}
	if err := w.recordFileEdit(ctx, task, targetRel, content, []byte(updated), "link_injection", "link task "+task.ID); err != nil {
		return w.failTask(ctx, taskID, attempts, task.CreatedAt, &logLines, fmt.Errorf("record file edit: %w", err))
	}

	genPos, _ := builder.FindAnchorInBody(updated, task.AnchorText, true)
	if genPos < 0 {
		genPos = 0
	}
	foundLocation := fmt.Sprintf("%s:%d", targetRel, lineNumber(updated, genPos))

	statusGenerated := "generated"
	completedAt := sql.NullTime{Time: w.Now(), Valid: true}
	clearErr := sql.NullString{}
	if err := w.Tasks.Update(ctx, taskID, sqlstore.LinkTaskUpdates{
		Status:           &statusGenerated,
		GeneratedContent: &sql.NullString{String: generated, Valid: true},
		FoundLocation:    &sql.NullString{String: foundLocation, Valid: true},
		ErrorMessage:     &clearErr,
		Attempts:         &attempts,
		CompletedAt:      &completedAt,
	}); err != nil {
		return fmt.Errorf("update task generated: %w", err)
	}
	if err := w.updateDomainLinkState(ctx, domain.ID, statusGenerated, task.ID, targetRel, task.AnchorText); err != nil {
		return w.failTask(ctx, taskID, attempts, task.CreatedAt, &logLines, err)
	}
	w.appendLog(ctx, taskID, &logLines, fmt.Sprintf("сгенерирован контент и вставлен в %s", targetRel))

	return nil
}

func (w *LinkWorker) loadPreviousTask(ctx context.Context, task *sqlstore.LinkTask, domain sqlstore.Domain) *sqlstore.LinkTask {
	if !domain.LinkLastTaskID.Valid {
		return nil
	}
	prevID := strings.TrimSpace(domain.LinkLastTaskID.String)
	if prevID == "" || prevID == task.ID {
		return nil
	}
	prev, err := w.Tasks.Get(ctx, prevID)
	if err != nil {
		return nil
	}
	return prev
}

func (w *LinkWorker) completeIfLinkExists(ctx context.Context, task *sqlstore.LinkTask, domain sqlstore.Domain, domainDir string, htmlFiles []string, attempts int, logLines *[]string) (bool, error) {
	for _, rel := range htmlFiles {
		full := filepath.Join(domainDir, filepath.FromSlash(rel))
		content, err := os.ReadFile(full)
		if err != nil {
			return false, fmt.Errorf("read html failed: %w", err)
		}
		pos, found := linkbuilder.FindExistingLinkInBody(string(content), task.AnchorText, task.TargetURL)
		if !found {
			continue
		}
		foundLocation := fmt.Sprintf("%s:%d", rel, lineNumber(string(content), pos))
		statusInserted := "inserted"
		completedAt := sql.NullTime{Time: w.Now(), Valid: true}
		clearErr := sql.NullString{}
		if err := w.Tasks.Update(ctx, task.ID, sqlstore.LinkTaskUpdates{
			Status:        &statusInserted,
			FoundLocation: &sql.NullString{String: foundLocation, Valid: true},
			ErrorMessage:  &clearErr,
			Attempts:      &attempts,
			CompletedAt:   &completedAt,
		}); err != nil {
			return false, fmt.Errorf("update task existing link: %w", err)
		}
		if err := w.updateDomainLinkState(ctx, domain.ID, statusInserted, task.ID, rel, task.AnchorText); err != nil {
			return false, err
		}
		w.appendLog(ctx, task.ID, logLines, fmt.Sprintf("ссылка уже существует в %s", rel))
		return true, nil
	}
	return false, nil
}

func (w *LinkWorker) replaceInFile(ctx context.Context, task *sqlstore.LinkTask, prevTask *sqlstore.LinkTask, attempts int, domain sqlstore.Domain, domainDir, rel string, builder *linkbuilder.Builder, logLines *[]string) (bool, error) {
	full := filepath.Join(domainDir, filepath.FromSlash(rel))
	if err := ensureWithinDir(domainDir, full); err != nil {
		return false, err
	}
	content, err := os.ReadFile(full)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return false, nil
		}
		return false, fmt.Errorf("read html failed: %w", err)
	}
	updated, replaced, removedTags := replaceLinkInContent(string(content), prevTask.AnchorText, prevTask.TargetURL, task.AnchorText, task.TargetURL)
	if !replaced && len(removedTags) == 0 {
		return false, nil
	}
	if err := os.WriteFile(full, []byte(updated), 0o644); err != nil {
		return false, fmt.Errorf("save html failed: %w", err)
	}
	if err := w.recordFileEdit(ctx, task, rel, content, []byte(updated), "link_injection", "link task "+task.ID); err != nil {
		return false, fmt.Errorf("record file edit: %w", err)
	}
	if len(removedTags) > 0 {
		for _, tag := range removedTags {
			w.appendLog(ctx, task.ID, logLines, fmt.Sprintf("удалена ссылка из %s в %s", tag, rel))
		}
	}
	if !replaced {
		return false, nil
	}
	pos, _ := builder.FindAnchorInBody(updated, task.AnchorText, true)
	if pos < 0 {
		pos = 0
	}
	foundLocation := fmt.Sprintf("%s:%d", rel, lineNumber(updated, pos))
	statusInserted := "inserted"
	completedAt := sql.NullTime{Time: w.Now(), Valid: true}
	clearErr := sql.NullString{}
	if err := w.Tasks.Update(ctx, task.ID, sqlstore.LinkTaskUpdates{
		Status:        &statusInserted,
		FoundLocation: &sql.NullString{String: foundLocation, Valid: true},
		ErrorMessage:  &clearErr,
		Attempts:      &attempts,
		CompletedAt:   &completedAt,
	}); err != nil {
		return false, fmt.Errorf("update task inserted: %w", err)
	}
	if err := w.updateDomainLinkState(ctx, domain.ID, statusInserted, task.ID, rel, task.AnchorText); err != nil {
		return false, err
	}
	w.appendLog(ctx, task.ID, logLines, fmt.Sprintf("обновлена ссылка в %s", rel))
	return true, nil
}

func (w *LinkWorker) replaceAcrossFiles(ctx context.Context, task *sqlstore.LinkTask, prevTask *sqlstore.LinkTask, attempts int, domain sqlstore.Domain, domainDir string, htmlFiles []string, builder *linkbuilder.Builder, logLines *[]string) (bool, error) {
	for _, rel := range htmlFiles {
		replaced, err := w.replaceInFile(ctx, task, prevTask, attempts, domain, domainDir, rel, builder, logLines)
		if err != nil {
			return false, err
		}
		if replaced {
			return true, nil
		}
	}
	return false, nil
}

func (w *LinkWorker) processRemoveTask(ctx context.Context, task *sqlstore.LinkTask, domain sqlstore.Domain, domainDir string, htmlFiles []string, attempts int, logLines *[]string) error {
	anchor := strings.TrimSpace(task.AnchorText)
	target := strings.TrimSpace(task.TargetURL)
	prevTask := w.loadPreviousTask(ctx, task, domain)
	if prevTask != nil {
		if strings.TrimSpace(prevTask.AnchorText) != "" {
			anchor = prevTask.AnchorText
		}
		if strings.TrimSpace(prevTask.TargetURL) != "" {
			target = prevTask.TargetURL
		}
	}
	if anchor == "" || target == "" {
		return w.failTask(ctx, task.ID, attempts, task.CreatedAt, logLines, errLinkSettingsNotConfigured)
	}

	candidates := make([]string, 0, 2)
	if domain.LinkFilePath.Valid && strings.TrimSpace(domain.LinkFilePath.String) != "" {
		candidates = append(candidates, strings.TrimSpace(domain.LinkFilePath.String))
	}
	if prevTask != nil && prevTask.FoundLocation.Valid {
		if rel := parseFoundLocation(prevTask.FoundLocation.String); rel != "" {
			candidates = append(candidates, rel)
		}
	}

	checked := map[string]struct{}{}
	for _, rel := range candidates {
		if _, ok := checked[rel]; ok {
			continue
		}
		checked[rel] = struct{}{}
		if removed, err := w.removeInFile(ctx, task, anchor, target, attempts, domain, domainDir, rel, logLines); err != nil {
			return w.failTask(ctx, task.ID, attempts, task.CreatedAt, logLines, err)
		} else if removed {
			return nil
		}
	}

	for _, rel := range htmlFiles {
		if _, ok := checked[rel]; ok {
			continue
		}
		if removed, err := w.removeInFile(ctx, task, anchor, target, attempts, domain, domainDir, rel, logLines); err != nil {
			return w.failTask(ctx, task.ID, attempts, task.CreatedAt, logLines, err)
		} else if removed {
			return nil
		}
	}

	w.appendLog(ctx, task.ID, logLines, "WARNING: ссылка не найдена, задача помечена как удаленная (идемпотентно)")
	statusRemoved := "removed"
	completedAt := sql.NullTime{Time: w.Now(), Valid: true}
	nullStr := sql.NullString{}
	if err := w.Tasks.Update(ctx, task.ID, sqlstore.LinkTaskUpdates{
		Status:           &statusRemoved,
		FoundLocation:    &nullStr,
		GeneratedContent: &nullStr,
		ErrorMessage:     &nullStr,
		Attempts:         &attempts,
		CompletedAt:      &completedAt,
	}); err != nil {
		return fmt.Errorf("update task removed without match: %w", err)
	}
	if err := w.updateDomainLinkState(ctx, domain.ID, statusRemoved, task.ID, "", ""); err != nil {
		return err
	}
	return nil
}

func (w *LinkWorker) removeInFile(ctx context.Context, task *sqlstore.LinkTask, anchor string, target string, attempts int, domain sqlstore.Domain, domainDir string, rel string, logLines *[]string) (bool, error) {
	full := filepath.Join(domainDir, filepath.FromSlash(rel))
	if err := ensureWithinDir(domainDir, full); err != nil {
		return false, err
	}
	content, err := os.ReadFile(full)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return false, nil
		}
		return false, fmt.Errorf("read html failed: %w", err)
	}
	updated, removed, removedTags, pos := removeLinkInContent(string(content), anchor, target)
	if !removed && len(removedTags) == 0 {
		return false, nil
	}
	if err := os.WriteFile(full, []byte(updated), 0o644); err != nil {
		return false, fmt.Errorf("save html failed: %w", err)
	}
	if err := w.recordFileEdit(ctx, task, rel, content, []byte(updated), "link_injection", "link remove "+task.ID); err != nil {
		return false, fmt.Errorf("record file edit: %w", err)
	}
	for _, tag := range removedTags {
		w.appendLog(ctx, task.ID, logLines, fmt.Sprintf("удалена ссылка из %s в %s", tag, rel))
	}

	foundLocation := sql.NullString{}
	if pos >= 0 {
		foundLocation = sql.NullString{String: fmt.Sprintf("%s:%d", rel, lineNumber(string(content), pos)), Valid: true}
	}
	statusRemoved := "removed"
	completedAt := sql.NullTime{Time: w.Now(), Valid: true}
	clearErr := sql.NullString{}
	if err := w.Tasks.Update(ctx, task.ID, sqlstore.LinkTaskUpdates{
		Status:           &statusRemoved,
		FoundLocation:    &foundLocation,
		GeneratedContent: &clearErr,
		ErrorMessage:     &clearErr,
		Attempts:         &attempts,
		CompletedAt:      &completedAt,
	}); err != nil {
		return false, fmt.Errorf("update task removed: %w", err)
	}
	if err := w.updateDomainLinkState(ctx, domain.ID, statusRemoved, task.ID, "", ""); err != nil {
		return false, err
	}
	w.appendLog(ctx, task.ID, logLines, fmt.Sprintf("ссылка удалена в %s", rel))
	return true, nil
}

func (w *LinkWorker) updateDomainLinkState(ctx context.Context, domainID string, status string, taskID string, filePath string, anchorSnapshot string) error {
	if w.Domains == nil {
		return nil
	}
	if err := w.Domains.UpdateLinkState(ctx, domainID, status, taskID, filePath, anchorSnapshot); err != nil {
		return fmt.Errorf("update domain link state: %w", err)
	}
	return nil
}

func (w *LinkWorker) failTask(ctx context.Context, taskID string, attempts int, createdAt time.Time, logLines *[]string, cause error) error {
	status := "failed"
	msg := sanitizeError(cause)
	now := w.Now
	if now == nil {
		now = func() time.Time { return time.Now().UTC() }
	}
	if logLines != nil && msg != "" {
		line := fmt.Sprintf("%s ERROR %s", now().Format(time.RFC3339), msg)
		*logLines = append(*logLines, line)
	}

	scheduledFor := time.Time{}
	completed := sql.NullTime{Time: now(), Valid: true}
	if isRetryableLinkError(cause, msg) {
		if createdAt.IsZero() {
			createdAt = now()
		}
		if next, err := retry.NextRetryAt(attempts, createdAt, now()); err == nil {
			status = "pending"
			scheduledFor = next
			completed = sql.NullTime{}
			if logLines != nil {
				*logLines = append(*logLines, fmt.Sprintf("%s retry scheduled at %s (attempt %d/%d)", now().Format(time.RFC3339), next.Format(time.RFC3339), attempts, retry.MaxRetries))
			}
		}
	}

	updates := sqlstore.LinkTaskUpdates{
		Status:       &status,
		ErrorMessage: &sql.NullString{String: msg, Valid: msg != ""},
		Attempts:     &attempts,
		CompletedAt:  &completed,
	}
	if !scheduledFor.IsZero() {
		updates.ScheduledFor = &scheduledFor
	}
	if logLines != nil {
		updates.LogLines = logLines
	}
	if w.Tasks != nil {
		_ = w.Tasks.Update(ctx, taskID, updates)
	}
	w.updateDomainStatusByTask(ctx, taskID, status)
	return nil
}

func (w *LinkWorker) updateDomainStatusByTask(ctx context.Context, taskID string, status string) {
	if w.Tasks == nil || w.Domains == nil {
		return
	}
	task, err := w.Tasks.Get(ctx, taskID)
	if err != nil || task == nil {
		return
	}
	if strings.TrimSpace(task.DomainID) == "" {
		return
	}
	_ = w.Domains.UpdateLinkStatus(ctx, task.DomainID, status)
}

func isRetryableLinkError(cause error, msg string) bool {
	if cause != nil {
		switch {
		case errors.Is(cause, errLinkSettingsNotConfigured):
			return false
		case errors.Is(cause, errAnchorTextRequired):
			return false
		case errors.Is(cause, errTargetURLRequired):
			return false
		case errors.Is(cause, errNoHTMLFiles):
			return false
		case errors.Is(cause, errRelinkSourceNotFound):
			return false
		case errors.Is(cause, errUnsupportedLinkAction):
			return false
		}
	}
	if msg == "" {
		return false
	}
	lower := strings.ToLower(msg)
	if strings.Contains(lower, "link settings not configured") {
		return false
	}
	if strings.Contains(lower, "anchor text is required") {
		return false
	}
	if strings.Contains(lower, "target url is required") {
		return false
	}
	if strings.Contains(lower, "no html files found") {
		return false
	}
	if strings.Contains(lower, "relink source not found") {
		return false
	}
	if strings.Contains(lower, "unsupported link task action") {
		return false
	}
	return true
}

func (w *LinkWorker) appendLog(ctx context.Context, taskID string, logLines *[]string, message string) {
	if logLines == nil {
		return
	}
	msg := strings.TrimSpace(message)
	if msg == "" {
		return
	}
	now := w.Now
	if now == nil {
		now = func() time.Time { return time.Now().UTC() }
	}
	line := fmt.Sprintf("%s %s", now().Format(time.RFC3339), msg)
	*logLines = append(*logLines, line)
	if w.Tasks == nil {
		return
	}
	if err := w.Tasks.Update(ctx, taskID, sqlstore.LinkTaskUpdates{LogLines: logLines}); err != nil && w.Logger != nil {
		w.Logger.Debugw("link task log update failed", "task_id", taskID, "error", err)
	}
}

func (w *LinkWorker) generateContent(ctx context.Context, task *sqlstore.LinkTask, domain sqlstore.Domain, pageContext string) (string, error) {
	if strings.TrimSpace(task.AnchorText) == "" {
		return "", errAnchorTextRequired
	}
	if strings.TrimSpace(task.TargetURL) == "" {
		return "", errTargetURLRequired
	}
	if w.Generator != nil {
		return w.Generator.Generate(ctx, task.AnchorText, task.TargetURL, pageContext)
	}
	apiKey, keyOwnerEmail, keyType, err := w.selectAPIKey(ctx, task, domain)
	if err != nil {
		return "", err
	}
	llmClient := llm.NewClient(llm.Config{
		APIKey:          apiKey,
		DefaultModel:    w.Config.GeminiDefaultModel,
		MaxRetries:      w.Config.GeminiMaxRetries,
		RetryDelay:      w.Config.GeminiRetryDelay,
		RequestTimeout:  w.Config.GeminiRequestTimeout,
		RateLimitPerMin: w.Config.GeminiRateLimitPerMin,
	})

	ctxSnippet := pageContext
	if len(ctxSnippet) > 2000 {
		ctxSnippet = ctxSnippet[:2000]
	}

	prompt := fmt.Sprintf(`You are an SEO copywriter. Generate exactly one HTML <p> paragraph that fits the page style.
Include the anchor text as a hyperlink to the target URL exactly once.
Return only the HTML snippet, no markdown or code fences.

Anchor text: %q
Target URL: %q
Page context (excerpt):
%s`, task.AnchorText, task.TargetURL, ctxSnippet)

	resp, err := llmClient.Generate(ctx, "link_task", prompt, "")
	reqs := llmClient.GetRequests()
	if len(reqs) > 0 {
		req := reqs[len(reqs)-1]
		w.logLLMUsage(ctx, req, task, domain, keyOwnerEmail, keyType)
	} else if err != nil {
		fallbackErr := llm.SanitizeError(err)
		fallbackReq := llm.LLMRequest{
			Stage:       "link_task",
			Model:       strings.TrimSpace(w.Config.GeminiDefaultModel),
			TokenSource: "estimated",
			Timestamp:   time.Now().UTC(),
		}
		if fallbackErr != nil {
			fallbackReq.Error = fallbackErr.Error()
		} else {
			fallbackReq.Error = strings.TrimSpace(err.Error())
		}
		w.logLLMUsage(ctx, fallbackReq, task, domain, keyOwnerEmail, keyType)
	}
	if err != nil {
		return "", err
	}

	clean := strings.TrimSpace(stripCodeFences(resp))
	if clean == "" {
		clean = defaultGenerated(task.AnchorText, task.TargetURL)
	}
	if !strings.Contains(clean, task.AnchorText) || !strings.Contains(clean, task.TargetURL) {
		clean = defaultGenerated(task.AnchorText, task.TargetURL)
	}
	return clean, nil
}

func (w *LinkWorker) selectAPIKey(ctx context.Context, task *sqlstore.LinkTask, domain sqlstore.Domain) (string, string, string, error) {
	apiKey := w.Config.GeminiAPIKey
	keyOwnerEmail := ""
	keyType := "global"
	if w.Users == nil || w.Projects == nil {
		if strings.TrimSpace(apiKey) == "" {
			return "", "", "", errors.New("API key not configured")
		}
		return apiKey, keyOwnerEmail, keyType, nil
	}
	project, err := w.Projects.GetByID(ctx, domain.ProjectID)
	if err != nil {
		return "", "", "", fmt.Errorf("project not found: %w", err)
	}

	tryUser := func(email string) (string, bool) {
		if strings.TrimSpace(email) == "" {
			return "", false
		}
		enc, _, err := w.Users.GetAPIKey(ctx, email)
		if err != nil || len(enc) == 0 {
			return "", false
		}
		keySecret := secretbox.DeriveKey(w.Config.APIKeySecret)
		decrypted, err := secretbox.Decrypt(keySecret, enc)
		if err != nil {
			return "", false
		}
		return string(decrypted), true
	}

	if key, ok := tryUser(task.CreatedBy); ok {
		return key, task.CreatedBy, "user", nil
	}
	if task.CreatedBy != project.UserEmail {
		if key, ok := tryUser(project.UserEmail); ok {
			return key, project.UserEmail, "user", nil
		}
	}
	if strings.TrimSpace(apiKey) == "" {
		return "", "", "", errors.New("API key not configured")
	}
	return apiKey, keyOwnerEmail, keyType, nil
}

func (w *LinkWorker) logLLMUsage(
	ctx context.Context,
	req llm.LLMRequest,
	task *sqlstore.LinkTask,
	domain sqlstore.Domain,
	keyOwnerEmail string,
	keyType string,
) {
	if w.LLMUsage == nil || task == nil {
		return
	}
	var (
		inputPrice  sql.NullFloat64
		outputPrice sql.NullFloat64
		estCost     sql.NullFloat64
	)
	if w.Pricing != nil {
		at := req.Timestamp
		if at.IsZero() {
			at = time.Now().UTC()
		}
		if pricing, err := w.Pricing.GetActiveByModel(ctx, "gemini", req.Model, at); err == nil && pricing != nil {
			inputPrice = sql.NullFloat64{Float64: pricing.InputUSDPerMillion, Valid: true}
			outputPrice = sql.NullFloat64{Float64: pricing.OutputUSDPerMillion, Valid: true}
			cost := estimateLLMCostUSD(req.PromptTokens, req.CompletionTokens, pricing.InputUSDPerMillion, pricing.OutputUSDPerMillion)
			estCost = sql.NullFloat64{Float64: cost, Valid: true}
		}
	}

	event := sqlstore.LLMUsageEvent{
		Provider:                 "gemini",
		Operation:                "link_ai_generate",
		Stage:                    sql.NullString{String: "link_task", Valid: true},
		Model:                    req.Model,
		Status:                   llmUsageStatus(req.Error),
		RequesterEmail:           strings.TrimSpace(task.CreatedBy),
		KeyOwnerEmail:            sql.NullString{String: strings.TrimSpace(keyOwnerEmail), Valid: strings.TrimSpace(keyOwnerEmail) != ""},
		KeyType:                  sql.NullString{String: strings.TrimSpace(keyType), Valid: strings.TrimSpace(keyType) != ""},
		ProjectID:                sql.NullString{String: domain.ProjectID, Valid: domain.ProjectID != ""},
		DomainID:                 sql.NullString{String: domain.ID, Valid: domain.ID != ""},
		LinkTaskID:               sql.NullString{String: task.ID, Valid: task.ID != ""},
		PromptTokens:             sql.NullInt64{Int64: req.PromptTokens, Valid: true},
		CompletionTokens:         sql.NullInt64{Int64: req.CompletionTokens, Valid: true},
		TotalTokens:              sql.NullInt64{Int64: req.TotalTokens, Valid: true},
		TokenSource:              llmTokenSource(req.TokenSource),
		InputPriceUSDPerMillion:  inputPrice,
		OutputPriceUSDPerMillion: outputPrice,
		EstimatedCostUSD:         estCost,
		ErrorMessage:             sql.NullString{String: req.Error, Valid: strings.TrimSpace(req.Error) != ""},
		CreatedAt:                req.Timestamp,
	}
	if strings.TrimSpace(event.RequesterEmail) == "" {
		// Без валидного requester_email не сможем сохранить событие из-за FK.
		return
	}
	if event.CreatedAt.IsZero() {
		event.CreatedAt = time.Now().UTC()
	}
	_ = w.LLMUsage.CreateEvent(ctx, event)
}

func estimateLLMCostUSD(promptTokens, completionTokens int64, inputUSDPerMillion, outputUSDPerMillion float64) float64 {
	promptCost := (float64(promptTokens) / 1_000_000.0) * inputUSDPerMillion
	completionCost := (float64(completionTokens) / 1_000_000.0) * outputUSDPerMillion
	return promptCost + completionCost
}

func llmUsageStatus(errText string) string {
	if strings.TrimSpace(errText) == "" {
		return "success"
	}
	return "error"
}

func llmTokenSource(src string) string {
	src = strings.TrimSpace(strings.ToLower(src))
	switch src {
	case "provider", "estimated", "mixed":
		return src
	default:
		return "estimated"
	}
}

func (w *LinkWorker) recordFileEdit(ctx context.Context, task *sqlstore.LinkTask, relPath string, before, after []byte, editType string, description string) error {
	if w.SiteFiles == nil || w.FileEdits == nil {
		return errors.New("file stores not configured")
	}

	file, err := w.SiteFiles.GetByPath(ctx, task.DomainID, relPath)
	if err != nil {
		if !errors.Is(err, sql.ErrNoRows) {
			return err
		}
		file = &sqlstore.SiteFile{
			ID:          uuid.NewString(),
			DomainID:    task.DomainID,
			Path:        relPath,
			ContentHash: sql.NullString{String: hashContent(before), Valid: true},
			SizeBytes:   int64(len(before)),
			MimeType:    detectMimeType(relPath, before),
		}
		if err := w.SiteFiles.Create(ctx, *file); err != nil {
			return err
		}
	}

	if err := w.SiteFiles.Update(ctx, file.ID, after); err != nil {
		return err
	}

	beforeHash := hashContent(before)
	afterHash := hashContent(after)
	edit := sqlstore.FileEdit{
		ID:                uuid.NewString(),
		FileID:            file.ID,
		EditedBy:          task.CreatedBy,
		ContentBeforeHash: sql.NullString{String: beforeHash, Valid: beforeHash != ""},
		ContentAfterHash:  sql.NullString{String: afterHash, Valid: afterHash != ""},
		EditType:          editType,
		EditDescription:   sql.NullString{String: description, Valid: description != ""},
		CreatedAt:         w.Now(),
	}
	return w.FileEdits.Create(ctx, edit)
}

func (w *LinkWorker) ensureBaseDir() (string, error) {
	baseDir := strings.TrimSpace(w.BaseDir)
	if baseDir == "" {
		baseDir = "server"
	}
	info, err := os.Stat(baseDir)
	if err != nil {
		return "", fmt.Errorf("base dir not found: %w", err)
	}
	if !info.IsDir() {
		return "", fmt.Errorf("base dir is not a directory: %s", baseDir)
	}
	return baseDir, nil
}

func (w *LinkWorker) domainDir(baseDir, domainURL string) (string, error) {
	domain, err := sanitizeDomain(domainURL)
	if err != nil {
		return "", err
	}
	target := filepath.Join(baseDir, domain)
	if err := ensureWithinDir(baseDir, target); err != nil {
		return "", err
	}
	return target, nil
}

func listHTMLFiles(root string) ([]string, error) {
	var files []string
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			return nil
		}
		if d.Type()&os.ModeSymlink != 0 {
			return nil
		}
		ext := strings.ToLower(filepath.Ext(d.Name()))
		if ext != ".html" && ext != ".htm" {
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		files = append(files, rel)
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Strings(files)
	return files, nil
}

func selectTargetHTML(files []string) string {
	if len(files) == 0 {
		return ""
	}
	for _, f := range files {
		if strings.EqualFold(filepath.Base(f), "index.html") {
			return f
		}
	}
	return files[0]
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

func lineNumber(content string, position int) int {
	if position <= 0 {
		return 1
	}
	count := 1
	for i := 0; i < position && i < len(content); i++ {
		if content[i] == '\n' {
			count++
		}
	}
	return count
}

func hashContent(content []byte) string {
	if len(content) == 0 {
		return ""
	}
	sum := sha256.Sum256(content)
	return hex.EncodeToString(sum[:])
}

func detectMimeType(path string, content []byte) string {
	ext := strings.ToLower(filepath.Ext(path))
	if ext != "" {
		if t := mime.TypeByExtension(ext); t != "" {
			return t
		}
	}
	if len(content) > 0 {
		return http.DetectContentType(content)
	}
	return "application/octet-stream"
}

func sanitizeDomain(raw string) (string, error) {
	d := strings.TrimSpace(raw)
	if d == "" {
		return "", errors.New("domain is empty")
	}
	d = strings.TrimPrefix(d, "http://")
	d = strings.TrimPrefix(d, "https://")
	if idx := strings.IndexAny(d, "/?"); idx >= 0 {
		d = d[:idx]
	}
	if idx := strings.Index(d, ":"); idx >= 0 {
		d = d[:idx]
	}
	d = strings.TrimSuffix(d, "/")
	d = strings.TrimSpace(d)
	if d == "" {
		return "", errors.New("domain is empty")
	}
	if strings.Contains(d, "..") {
		return "", errors.New("domain contains invalid sequence")
	}
	if strings.ContainsAny(d, "\\") {
		return "", errors.New("domain contains invalid separator")
	}
	if strings.ContainsAny(d, " \t\n\r") {
		return "", errors.New("domain contains whitespace")
	}
	return d, nil
}

func ensureWithinDir(baseDir, target string) error {
	baseAbs, err := filepath.Abs(baseDir)
	if err != nil {
		return err
	}
	targetAbs, err := filepath.Abs(target)
	if err != nil {
		return err
	}
	baseAbs = filepath.Clean(baseAbs)
	if targetAbs == baseAbs {
		return errors.New("path equals base dir")
	}
	if !strings.HasPrefix(targetAbs, baseAbs+string(os.PathSeparator)) {
		return errors.New("path escapes base dir")
	}
	return nil
}

func stripCodeFences(text string) string {
	out := strings.TrimSpace(text)
	if !strings.Contains(out, "```") {
		return out
	}
	lines := strings.Split(out, "\n")
	var trimmed []string
	inFence := false
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "```") {
			inFence = !inFence
			continue
		}
		if inFence {
			trimmed = append(trimmed, line)
		}
	}
	if len(trimmed) == 0 {
		return strings.ReplaceAll(out, "```", "")
	}
	return strings.Join(trimmed, "\n")
}

func defaultGenerated(anchorText, targetURL string) string {
	return fmt.Sprintf("<p><a href=\"%s\">%s</a></p>", targetURL, anchorText)
}

func replaceLinkInContent(html string, oldAnchor string, oldTarget string, newAnchor string, newTarget string) (string, bool, []string) {
	oldAnchor = strings.TrimSpace(oldAnchor)
	oldTarget = strings.TrimSpace(oldTarget)
	newAnchor = strings.TrimSpace(newAnchor)
	newTarget = strings.TrimSpace(newTarget)
	if oldAnchor == "" || oldTarget == "" || newAnchor == "" || newTarget == "" {
		return html, false, nil
	}
	cleaned, removedTags := stripAnchorsInDisallowedTags(html, oldAnchor, oldTarget)
	if !strings.EqualFold(oldAnchor, newAnchor) || !strings.EqualFold(oldTarget, newTarget) {
		var extra []string
		cleaned, extra = stripAnchorsInDisallowedTags(cleaned, newAnchor, newTarget)
		if len(extra) > 0 {
			removedTags = append(removedTags, extra...)
			removedTags = uniqueStrings(removedTags)
		}
	}
	updated, replaced := replaceLinkInBody(cleaned, oldAnchor, oldTarget, newAnchor, newTarget)
	if replaced {
		return updated, true, removedTags
	}
	return cleaned, false, removedTags
}

func replaceLinkInBody(html string, oldAnchor string, oldTarget string, newAnchor string, newTarget string) (string, bool) {
	lower := strings.ToLower(html)
	bodyStart := strings.Index(lower, "<body")
	if bodyStart == -1 {
		return html, false
	}
	openEnd := strings.Index(lower[bodyStart:], ">")
	if openEnd == -1 {
		return html, false
	}
	bodyContentStart := bodyStart + openEnd + 1
	bodyEnd := strings.Index(lower[bodyContentStart:], "</body>")
	if bodyEnd == -1 {
		return html, false
	}
	bodyContentEnd := bodyContentStart + bodyEnd
	body := html[bodyContentStart:bodyContentEnd]
	targets := targetVariants(oldTarget)
	if len(targets) == 0 {
		return html, false
	}
	replacement := fmt.Sprintf("<a href=\"%s\">%s</a>", newTarget, newAnchor)
	for _, target := range targets {
		pattern := `(?is)<a\b[^>]*\bhref\s*=\s*['"]` + regexp.QuoteMeta(target) + `['"][^>]*>\s*` + regexp.QuoteMeta(oldAnchor) + `\s*</a>`
		re, err := regexp.Compile(pattern)
		if err != nil {
			continue
		}
		if !re.MatchString(body) {
			continue
		}
		newBody := re.ReplaceAllString(body, replacement)
		updated := html[:bodyContentStart] + newBody + html[bodyContentEnd:]
		return updated, true
	}
	return html, false
}

func removeLinkInContent(html string, anchor string, target string) (string, bool, []string, int) {
	anchor = strings.TrimSpace(anchor)
	target = strings.TrimSpace(target)
	if anchor == "" || target == "" {
		return html, false, nil, -1
	}
	cleaned, removedTags := stripAnchorsInDisallowedTags(html, anchor, target)
	updated, removed, pos := removeLinkInBody(cleaned, anchor, target)
	if removed {
		return updated, true, removedTags, pos
	}
	if len(removedTags) > 0 {
		return cleaned, true, removedTags, -1
	}
	return cleaned, false, removedTags, -1
}

func removeLinkInBody(html string, anchor string, target string) (string, bool, int) {
	lower := strings.ToLower(html)
	bodyStart := strings.Index(lower, "<body")
	if bodyStart == -1 {
		return html, false, -1
	}
	openEnd := strings.Index(lower[bodyStart:], ">")
	if openEnd == -1 {
		return html, false, -1
	}
	bodyContentStart := bodyStart + openEnd + 1
	bodyEnd := strings.Index(lower[bodyContentStart:], "</body>")
	if bodyEnd == -1 {
		return html, false, -1
	}
	bodyContentEnd := bodyContentStart + bodyEnd
	body := html[bodyContentStart:bodyContentEnd]
	targets := targetVariants(target)
	if len(targets) == 0 {
		return html, false, -1
	}
	for _, tgt := range targets {
		pattern := `(?is)<a\b[^>]*\bhref\s*=\s*['"]` + regexp.QuoteMeta(strings.TrimSpace(tgt)) + `['"][^>]*>\s*` + regexp.QuoteMeta(strings.TrimSpace(anchor)) + `\s*</a>`
		re, err := regexp.Compile(pattern)
		if err != nil {
			continue
		}
		loc := re.FindStringIndex(body)
		if loc == nil {
			continue
		}
		newBody := re.ReplaceAllString(body, strings.TrimSpace(anchor))
		updated := html[:bodyContentStart] + newBody + html[bodyContentEnd:]
		return updated, true, bodyContentStart + loc[0]
	}
	return html, false, -1
}

func stripAnchorsInDisallowedTags(html string, anchorText string, targetURL string) (string, []string) {
	tags := []string{"title", "h1", "h2", "h3", "h4", "h5", "h6"}
	updated := html
	var removed []string
	for _, tag := range tags {
		reTag := regexp.MustCompile(`(?is)<` + tag + `\b[^>]*>.*?</` + tag + `>`)
		updated = reTag.ReplaceAllStringFunc(updated, func(block string) string {
			cleaned, didStrip := stripAnchors(block, anchorText, targetURL)
			if didStrip {
				removed = append(removed, tag)
			}
			return cleaned
		})
	}
	return updated, uniqueStrings(removed)
}

func stripAnchors(html string, anchorText string, targetURL string) (string, bool) {
	reAnchor := regexp.MustCompile(`(?is)<a\b[^>]*\bhref\s*=\s*['"]([^'"]+)['"][^>]*>(.*?)</a>`)
	changed := false
	updated := reAnchor.ReplaceAllStringFunc(html, func(match string) string {
		sub := reAnchor.FindStringSubmatch(match)
		if len(sub) < 3 {
			return match
		}
		href := strings.TrimSpace(sub[1])
		text := strings.TrimSpace(sub[2])
		if shouldStripAnchor(href, text, anchorText, targetURL) {
			changed = true
			return text
		}
		return match
	})
	return updated, changed
}

func shouldStripAnchor(href string, text string, anchorText string, targetURL string) bool {
	anchorText = strings.TrimSpace(anchorText)
	targetURL = strings.TrimSpace(targetURL)
	if anchorText == "" || targetURL == "" {
		return false
	}
	normalizedHref := normalizeURLForCompare(href)
	normalizedTarget := normalizeURLForCompare(targetURL)
	if strings.EqualFold(strings.TrimSpace(text), anchorText) && normalizedHref == normalizedTarget {
		return true
	}
	if strings.EqualFold(strings.TrimSpace(text), anchorText) {
		return true
	}
	if normalizedHref == normalizedTarget {
		return true
	}
	return false
}

func normalizeURLForCompare(raw string) string {
	raw = strings.TrimSpace(raw)
	raw = strings.TrimRight(raw, "/")
	raw = strings.TrimPrefix(raw, "https://")
	raw = strings.TrimPrefix(raw, "http://")
	return strings.ToLower(raw)
}

func targetVariants(target string) []string {
	target = strings.TrimSpace(target)
	if target == "" {
		return nil
	}
	trimmed := strings.TrimRight(target, "/")
	if trimmed == target {
		return []string{target}
	}
	return []string{target, trimmed}
}

func uniqueStrings(values []string) []string {
	if len(values) == 0 {
		return values
	}
	seen := map[string]struct{}{}
	out := make([]string, 0, len(values))
	for _, v := range values {
		if _, ok := seen[v]; ok {
			continue
		}
		seen[v] = struct{}{}
		out = append(out, v)
	}
	return out
}

func parseFoundLocation(found string) string {
	found = strings.TrimSpace(found)
	if found == "" {
		return ""
	}
	if idx := strings.LastIndex(found, ":"); idx > 0 {
		return found[:idx]
	}
	return found
}

func sanitizeError(err error) string {
	if err == nil {
		return ""
	}
	msg := err.Error()
	msg = strings.TrimSpace(msg)
	if len(msg) > 500 {
		msg = msg[:500]
	}
	return msg
}
