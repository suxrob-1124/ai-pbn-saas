package store

import (
	"encoding/json"
	"time"
)

// CheckpointData представляет структуру чекпоинта
type CheckpointData struct {
	Step          string                 `json:"step"`           // Текущий шаг
	StepProgress  int                    `json:"step_progress"`  // Прогресс внутри шага (0-100)
	Context       map[string]interface{} `json:"context"`        // Контекст для восстановления
	ArtifactsSnapshot map[string]interface{} `json:"artifacts_snapshot"` // Снимок artifacts
	CreatedAt     string                 `json:"created_at"`    // Время создания чекпоинта
}

// NewCheckpoint создает новый чекпоинт
func NewCheckpoint(step string, stepProgress int, context map[string]interface{}, artifacts map[string]interface{}) *CheckpointData {
	return &CheckpointData{
		Step:             step,
		StepProgress:     stepProgress,
		Context:          context,
		ArtifactsSnapshot: artifacts,
		CreatedAt:        time.Now().Format(time.RFC3339),
	}
}

// Marshal преобразует CheckpointData в JSON
func (c *CheckpointData) Marshal() ([]byte, error) {
	return json.Marshal(c)
}

// UnmarshalCheckpoint преобразует JSON в CheckpointData
func UnmarshalCheckpoint(data []byte) (*CheckpointData, error) {
	var cp CheckpointData
	if err := json.Unmarshal(data, &cp); err != nil {
		return nil, err
	}
	return &cp, nil
}

