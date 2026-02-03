package pipeline

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"obzornik-pbn-generator/internal/store"
	"obzornik-pbn-generator/internal/store/sqlstore"
)

// Специальные ошибки для внешней обработки
var (
	ErrPipelinePaused    = fmt.Errorf("pipeline paused")
	ErrPipelineCancelled = fmt.Errorf("pipeline cancelled")
)

// Step представляет один шаг в pipeline генерации
type Step interface {
	// Name возвращает имя шага (например, "serp_analysis")
	Name() string

	// ArtifactKey возвращает ключ артефакта, наличие которого означает, что шаг выполнен
	// Например, "analysis_csv" для SERP анализа
	ArtifactKey() string

	// Execute выполняет логику шага
	// Возвращает map[string]any с артефактами для сохранения
	Execute(ctx context.Context, state *PipelineState) (map[string]any, error)

	// Progress возвращает процент прогресса после выполнения шага (0-100)
	Progress() int
}

// PipelineState содержит состояние pipeline между шагами
type PipelineState struct {
	GenerationID string
	DomainID     string
	ForceStep    string // Если установлен, принудительно выполнить этот шаг и все последующие

	// Stores
	DomainStore     *sqlstore.DomainStore
	GenerationStore *sqlstore.GenerationStore
	PromptStore     *sqlstore.PromptStore
	ProjectStore    *sqlstore.ProjectStore

	// Services
	LLMClient     LLMClient
	PromptManager PromptManager
	Analyzer      Analyzer

	// Config
	DefaultModel string // Дефолтная модель LLM

	// Domain data
	Domain  *sqlstore.Domain
	Project *sqlstore.Project

	// Accumulated artifacts (из БД + новые)
	Artifacts map[string]any

	// Logging
	AppendLog func(string)
	Logs      *[]string

	// Context for checkpoint saving
	Context map[string]any
}

// Pipeline управляет выполнением шагов генерации
type Pipeline struct {
	steps []Step
	state *PipelineState

	// Функция для проверки актуального статуса генерации в БД
	CheckStatus func() (string, error)
}

// NewPipeline создает новый pipeline с заданными шагами
func NewPipeline(steps []Step, state *PipelineState) *Pipeline {
	return &Pipeline{
		steps: steps,
		state: state,
	}
}

// Run выполняет все шаги pipeline с умным пропуском
func (p *Pipeline) Run(ctx context.Context) error {
	// Загружаем существующие artifacts из БД
	if err := p.loadArtifacts(ctx); err != nil {
		return fmt.Errorf("failed to load artifacts: %w", err)
	}

	if p.state.AppendLog != nil {
		keys := make([]string, 0, len(p.state.Artifacts))
		for k := range p.state.Artifacts {
			keys = append(keys, k)
		}
		p.state.AppendLog(fmt.Sprintf("Available artifacts: %v", keys))
	}

	forceFromIndex := -1
	forcingActive := false
	if p.state.ForceStep != "" {
		for i, step := range p.steps {
			if step.Name() == p.state.ForceStep {
				forceFromIndex = i
				p.state.AppendLog(fmt.Sprintf("Force step enabled: starting from %s", p.state.ForceStep))
				break
			}
		}
	}

	// Выполняем шаги по порядку
	for i, step := range p.steps {
		stepName := step.Name()
		artifactKey := step.ArtifactKey()

		// Дополнительная проверка прерываний между шагами
		if p.CheckStatus != nil {
			status, err := p.CheckStatus()
			if err != nil {
				return fmt.Errorf("failed to check status before step %s: %w", stepName, err)
			}
			switch status {
			case "pause_requested":
				p.state.AppendLog("Pause requested, stopping pipeline")
				// Сохраняем чекпоинт и переводим задачу в paused
				if err := p.saveCheckpoint(ctx, stepName, step.Progress()); err != nil {
					p.state.AppendLog(fmt.Sprintf("failed to save checkpoint on pause: %v", err))
				}
				_ = p.state.GenerationStore.UpdateStatus(ctx, p.state.GenerationID, "paused", step.Progress(), nil)
				_ = p.state.DomainStore.UpdateStatus(ctx, p.state.DomainID, "waiting")
				return ErrPipelinePaused
			case "cancelling":
				p.state.AppendLog("Cancellation requested, stopping pipeline")
				_ = p.state.GenerationStore.UpdateStatus(ctx, p.state.GenerationID, "cancelled", step.Progress(), nil)
				_ = p.state.DomainStore.UpdateStatus(ctx, p.state.DomainID, "waiting")
				return ErrPipelineCancelled
			}
		}

		if forceFromIndex != -1 && i >= forceFromIndex {
			forcingActive = true
		}

		exists := false
		if _, ok := p.state.Artifacts[artifactKey]; ok {
			exists = true
		}
		p.state.AppendLog(fmt.Sprintf("Checking step skip: step=%s looking_for=%s exists=%v forcing=%v", stepName, artifactKey, exists, forcingActive))

		if !forcingActive {
			if _, exists := p.state.Artifacts[artifactKey]; exists {
				p.state.AppendLog(fmt.Sprintf("Step '%s' skipped: artifact '%s' already exists", stepName, artifactKey))
				if err := p.restoreFromArtifacts(stepName, artifactKey); err != nil {
					p.state.AppendLog(fmt.Sprintf("Warning: failed to restore from artifacts for step %s: %v", stepName, err))
				}
				continue
			}
		}

		// Выполняем шаг
		p.state.AppendLog(fmt.Sprintf("Executing step: %s", stepName))

		// Проверяем статус перед началом шага (для паузы/отмены)
		if err := p.checkStatusBeforeStep(ctx, stepName); err != nil {
			return err
		}

		artifacts, err := step.Execute(ctx, p.state)
		if err != nil {
			return fmt.Errorf("step %s failed: %w", stepName, err)
		}

		// Сохраняем артефакты
		for k, v := range artifacts {
			p.state.Artifacts[k] = v
		}

		// Сохраняем artifacts в БД
		if err := p.saveArtifacts(ctx); err != nil {
			return fmt.Errorf("failed to save artifacts: %w", err)
		}

		// Сохраняем чекпоинт
		progress := step.Progress()
		if err := p.saveCheckpoint(ctx, stepName, progress); err != nil {
			return fmt.Errorf("failed to save checkpoint: %w", err)
		}

		// Проверяем статус после завершения шага
		if err := p.checkStatusAfterStep(ctx, stepName, progress); err != nil {
			return err
		}

		// Обновляем прогресс
		if err := p.state.GenerationStore.UpdateStatus(ctx, p.state.GenerationID, "processing", progress, nil); err != nil {
			p.state.AppendLog(fmt.Sprintf("Warning: failed to update progress: %v", err))
		}

		p.state.AppendLog(fmt.Sprintf("Step '%s' completed successfully", stepName))
	}

	return nil
}

// loadArtifacts загружает существующие artifacts из БД
func (p *Pipeline) loadArtifacts(ctx context.Context) error {
	if p.state.Artifacts == nil {
		p.state.Artifacts = make(map[string]any)
	}
	gen, err := p.state.GenerationStore.Get(ctx, p.state.GenerationID)
	if err != nil {
		return err
	}

	if len(gen.Artifacts) > 0 {
		if err := json.Unmarshal(gen.Artifacts, &p.state.Artifacts); err != nil {
			return fmt.Errorf("failed to unmarshal artifacts: %w", err)
		}
	}

	return nil
}

// saveArtifacts сохраняет artifacts в БД
func (p *Pipeline) saveArtifacts(ctx context.Context) error {
	artifactsJSON, err := json.Marshal(p.state.Artifacts)
	if err != nil {
		return fmt.Errorf("failed to marshal artifacts: %w", err)
	}

	// Обновляем только artifacts, не трогая другие поля
	gen, err := p.state.GenerationStore.Get(ctx, p.state.GenerationID)
	if err != nil {
		return err
	}

	var logsJSON []byte
	if p.state.Logs != nil && *p.state.Logs != nil {
		logsJSON, _ = json.Marshal(*p.state.Logs)
	}

	// Преобразуем sql.NullString в *string
	var errorPtr *string
	if gen.Error.Valid {
		errorPtr = &gen.Error.String
	}

	// Преобразуем sql.NullTime в *time.Time
	var startedAtPtr *time.Time
	if gen.StartedAt.Valid {
		startedAtPtr = &gen.StartedAt.Time
	}

	var finishedAtPtr *time.Time
	if gen.FinishedAt.Valid {
		finishedAtPtr = &gen.FinishedAt.Time
	}

	return p.state.GenerationStore.UpdateFull(
		ctx,
		p.state.GenerationID,
		gen.Status,
		gen.Progress,
		errorPtr,
		logsJSON,
		artifactsJSON,
		startedAtPtr,
		finishedAtPtr,
		nil, // promptID - не обновляем здесь
	)
}

// saveCheckpoint сохраняет чекпоинт для возможности паузы/возобновления
func (p *Pipeline) saveCheckpoint(ctx context.Context, stepName string, progress int) error {
	// Конвертируем context и artifacts в правильные типы для store.Checkpoint
	contextMap := make(map[string]interface{})
	for k, v := range p.state.Context {
		contextMap[k] = v
	}

	artifactsMap := make(map[string]interface{})
	for k, v := range p.state.Artifacts {
		artifactsMap[k] = v
	}

	cp := store.NewCheckpoint(stepName, progress, contextMap, artifactsMap)
	cpData, err := cp.Marshal()
	if err != nil {
		return fmt.Errorf("failed to marshal checkpoint: %w", err)
	}

	return p.state.GenerationStore.SaveCheckpoint(ctx, p.state.GenerationID, cpData)
}

// checkStatusBeforeStep проверяет статус перед началом шага (для паузы/отмены)
func (p *Pipeline) checkStatusBeforeStep(ctx context.Context, stepName string) error {
	shouldCont, shouldPause, shouldCancel, err := shouldContinue(ctx, p.state.GenerationStore, p.state.GenerationID)
	if err != nil {
		return err
	}

	if shouldCancel {
		p.state.AppendLog(fmt.Sprintf("Отмена запрошена на шаге %s", stepName))
		emptyArtifacts, _ := json.Marshal(map[string]interface{}{})
		logsJSON, _ := json.Marshal([]string{})
		now := time.Now()
		_ = p.state.GenerationStore.UpdateFull(ctx, p.state.GenerationID, "cancelled", 0, nil, logsJSON, emptyArtifacts, nil, &now, nil)
		_ = p.state.GenerationStore.ClearCheckpoint(ctx, p.state.GenerationID)
		_ = p.state.DomainStore.UpdateStatus(ctx, p.state.DomainID, "waiting")
		return fmt.Errorf("generation cancelled")
	}

	if shouldPause {
		p.state.AppendLog(fmt.Sprintf("Пауза запрошена на шаге %s, сохраняем чекпоинт", stepName))
		if err := p.saveCheckpoint(ctx, stepName, 0); err != nil {
			p.state.AppendLog(fmt.Sprintf("Ошибка сохранения чекпоинта: %v", err))
		}
		logsJSON, _ := json.Marshal([]string{})
		artBytes, _ := json.Marshal(p.state.Artifacts)
		gen, _ := p.state.GenerationStore.Get(ctx, p.state.GenerationID)
		_ = p.state.GenerationStore.UpdateFull(ctx, p.state.GenerationID, "paused", gen.Progress, nil, logsJSON, artBytes, nil, nil, nil)
		_ = p.state.DomainStore.UpdateStatus(ctx, p.state.DomainID, "waiting")
		return fmt.Errorf("generation paused")
	}

	if !shouldCont {
		return fmt.Errorf("cannot continue generation")
	}

	return nil
}

// checkStatusAfterStep проверяет статус после завершения шага
func (p *Pipeline) checkStatusAfterStep(ctx context.Context, stepName string, progress int) error {
	shouldCont, shouldPause, shouldCancel, err := shouldContinue(ctx, p.state.GenerationStore, p.state.GenerationID)
	if err != nil {
		return err
	}

	if shouldCancel {
		p.state.AppendLog(fmt.Sprintf("Отмена запрошена после шага %s", stepName))
		emptyArtifacts, _ := json.Marshal(map[string]interface{}{})
		logsJSON, _ := json.Marshal([]string{})
		now := time.Now()
		_ = p.state.GenerationStore.UpdateFull(ctx, p.state.GenerationID, "cancelled", 0, nil, logsJSON, emptyArtifacts, nil, &now, nil)
		_ = p.state.GenerationStore.ClearCheckpoint(ctx, p.state.GenerationID)
		_ = p.state.DomainStore.UpdateStatus(ctx, p.state.DomainID, "waiting")
		return fmt.Errorf("generation cancelled")
	}

	if shouldPause {
		p.state.AppendLog(fmt.Sprintf("Пауза запрошена после шага %s, сохраняем чекпоинт", stepName))
		if err := p.saveCheckpoint(ctx, stepName, progress); err != nil {
			p.state.AppendLog(fmt.Sprintf("Ошибка сохранения чекпоинта: %v", err))
		}
		logsJSON, _ := json.Marshal([]string{})
		artBytes, _ := json.Marshal(p.state.Artifacts)
		_ = p.state.GenerationStore.UpdateFull(ctx, p.state.GenerationID, "paused", progress, nil, logsJSON, artBytes, nil, nil, nil)
		_ = p.state.DomainStore.UpdateStatus(ctx, p.state.DomainID, "waiting")
		return fmt.Errorf("generation paused")
	}

	if !shouldCont {
		return fmt.Errorf("cannot continue generation")
	}

	return nil
}

// restoreFromArtifacts восстанавливает данные из artifacts в context для следующих шагов
// Это нужно, чтобы последующие шаги могли использовать результаты предыдущих
func (p *Pipeline) restoreFromArtifacts(stepName, artifactKey string) error {
	// Восстанавливаем специфичные для шага данные
	switch stepName {
	case StepSERPAnalysis:
		if serp, ok := p.state.Artifacts["serp_data"].(map[string]any); ok {
			p.state.Context["serp_data"] = serp
			if csv, ok := serp["analysis_csv"].(string); ok {
				p.state.Context["analysis_csv"] = csv
			}
			if contents, ok := serp["contents_txt"].(string); ok {
				p.state.Context["contents_txt"] = contents
			}
		} else {
			// обратная совместимость
			if csv, ok := p.state.Artifacts["analysis_csv"].(string); ok {
				p.state.Context["analysis_csv"] = csv
			}
			if contents, ok := p.state.Artifacts["contents_txt"].(string); ok {
				p.state.Context["contents_txt"] = contents
			}
		}
	case StepCompetitorAnalysis:
		if analysis, ok := p.state.Artifacts["competitor_analysis"].(string); ok {
			p.state.Context["competitor_analysis"] = analysis
			p.state.Context["llm_analysis"] = analysis
		} else if analysis, ok := p.state.Artifacts["llm_analysis"].(string); ok {
			p.state.Context["llm_analysis"] = analysis
			p.state.Context["competitor_analysis"] = analysis
		}
	case StepTechnicalSpec:
		// Восстанавливаем technical_spec
		if spec, ok := p.state.Artifacts["technical_spec"].(string); ok {
			p.state.Context["technical_spec"] = spec
		}
	case StepContentGeneration:
		// Восстанавливаем content_markdown
		if content, ok := p.state.Artifacts["content_markdown"].(string); ok {
			p.state.Context["content_markdown"] = content
		}
	case StepDesignArchitecture:
		// Восстанавливаем design_system
		if design, ok := p.state.Artifacts["design_system"].(string); ok {
			p.state.Context["design_system"] = design
		}
	case StepLogoGeneration:
		if logo, ok := p.state.Artifacts["logo_svg"].(string); ok {
			p.state.Context["logo_svg"] = logo
		}
	case StepHTMLGeneration:
		if html, ok := p.state.Artifacts["html_raw"].(string); ok {
			p.state.Context["html_raw"] = html
		}
	case StepCSSGeneration:
		if css, ok := p.state.Artifacts["css_content"].(string); ok {
			p.state.Context["css_content"] = css
		}
	case StepJSGeneration:
		if js, ok := p.state.Artifacts["js_content"].(string); ok {
			p.state.Context["js_content"] = js
		}
	}

	return nil
}
