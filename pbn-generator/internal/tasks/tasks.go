package tasks

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/hibiken/asynq"

	"obzornik-pbn-generator/internal/config"
)

const (
	TaskGenerate      = "generate:domain"
	TaskSchedulerTick = "scheduler:tick"
	TaskProcessLink   = "link:process"
)

type GeneratePayload struct {
	GenerationID string `json:"generation_id"`
	DomainID     string `json:"domain_id"`
	ForceStep    string `json:"force_step,omitempty"` // С какого шага начать принудительно (и все последующие)
}

// LinkTaskPayload описывает задачу линкбилдинга.
type LinkTaskPayload struct {
	TaskID string `json:"task_id"`
}

func NewGenerateTask(genID, domainID string, forceStep string) *asynq.Task {
	payload, _ := json.Marshal(GeneratePayload{
		GenerationID: genID,
		DomainID:     domainID,
		ForceStep:    forceStep,
	})
	return asynq.NewTask(TaskGenerate, payload, asynq.MaxRetry(5))
}

func NewSchedulerTickTask() *asynq.Task {
	return asynq.NewTask(TaskSchedulerTick, nil, asynq.MaxRetry(0))
}

// NewLinkTaskTask создает задачу линкбилдинга.
func NewLinkTaskTask(taskID string) *asynq.Task {
	payload, _ := json.Marshal(LinkTaskPayload{TaskID: taskID})
	return asynq.NewTask(TaskProcessLink, payload, asynq.MaxRetry(5))
}

type Enqueuer interface {
	Enqueue(ctx context.Context, task *asynq.Task, opts ...asynq.Option) (*asynq.TaskInfo, error)
}

type Client struct {
	*asynq.Client
}

func (c *Client) Enqueue(ctx context.Context, task *asynq.Task, opts ...asynq.Option) (*asynq.TaskInfo, error) {
	return c.EnqueueContext(ctx, task, opts...)
}

func NewClient(cfg config.Config) (*Client, error) {
	redisCfg := asynq.RedisClientOpt{
		Addr:     cfg.RedisAddr,
		Password: cfg.RedisPassword,
		DB:       cfg.RedisDB,
	}
	return &Client{Client: asynq.NewClient(redisCfg)}, nil
}

func NewServer(cfg config.Config, concurrency int) *asynq.Server {
	return asynq.NewServer(
		asynq.RedisClientOpt{
			Addr:     cfg.RedisAddr,
			Password: cfg.RedisPassword,
			DB:       cfg.RedisDB,
		},
		asynq.Config{
			Concurrency: concurrency,
			Queues: map[string]int{
				"default": 1,
			},
		},
	)
}

func ParseGeneratePayload(task *asynq.Task) (GeneratePayload, error) {
	var p GeneratePayload
	if err := json.Unmarshal(task.Payload(), &p); err != nil {
		return p, fmt.Errorf("invalid payload: %w", err)
	}
	if p.GenerationID == "" || p.DomainID == "" {
		return p, fmt.Errorf("missing ids")
	}
	return p, nil
}

// ParseLinkTaskPayload разбирает payload задачи линкбилдинга.
func ParseLinkTaskPayload(task *asynq.Task) (LinkTaskPayload, error) {
	var p LinkTaskPayload
	if err := json.Unmarshal(task.Payload(), &p); err != nil {
		return p, fmt.Errorf("invalid payload: %w", err)
	}
	if p.TaskID == "" {
		return p, fmt.Errorf("missing task id")
	}
	return p, nil
}
