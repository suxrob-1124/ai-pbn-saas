package tasks

import (
	"context"
	"encoding/json"
	"fmt"
	"hash/fnv"
	"strings"

	"github.com/hibiken/asynq"

	"obzornik-pbn-generator/internal/config"
)

const (
	TaskGenerate      = "generate:domain"
	TaskSchedulerTick = "scheduler:tick"
	TaskProcessLink   = "link:process"
	TaskProcessIndex  = "index:process"
	TaskLegacyImport  = "legacy:import"
)

const (
	genQueuePrefix  = "gen"
	linkQueuePrefix = "link"
	schedulerQueue  = "scheduler"
	defaultQueue    = "default"
)

type GeneratePayload struct {
	GenerationID   string `json:"generation_id"`
	DomainID       string `json:"domain_id"`
	ForceStep      string `json:"force_step,omitempty"`      // С какого шага начать принудительно (и все последующие)
	GenerationType string `json:"generation_type,omitempty"` // Тип генерации (single_page, webarchive_single, ...)
}

// LinkTaskPayload описывает задачу линкбилдинга.
type LinkTaskPayload struct {
	TaskID string `json:"task_id"`
}

type IndexCheckPayload struct {
	CheckID string `json:"check_id"`
}

type LegacyImportPayload struct {
	JobID    string `json:"job_id"`
	ItemID   string `json:"item_id"`
	DomainID string `json:"domain_id"`
}

func NewGenerateTask(genID, domainID string, forceStep string, generationType string) *asynq.Task {
	payload, _ := json.Marshal(GeneratePayload{
		GenerationID:   genID,
		DomainID:       domainID,
		ForceStep:      forceStep,
		GenerationType: generationType,
	})
	return asynq.NewTask(TaskGenerate, payload, asynq.MaxRetry(5))
}

func NewSchedulerTickTask() *asynq.Task {
	return asynq.NewTask(TaskSchedulerTick, nil, asynq.MaxRetry(0), asynq.Queue(schedulerQueue))
}

// NewLinkTaskTask создает задачу линкбилдинга.
func NewLinkTaskTask(taskID string) *asynq.Task {
	payload, _ := json.Marshal(LinkTaskPayload{TaskID: taskID})
	return asynq.NewTask(TaskProcessLink, payload, asynq.MaxRetry(5))
}

func NewIndexCheckTask(checkID string) *asynq.Task {
	payload, _ := json.Marshal(IndexCheckPayload{CheckID: checkID})
	return asynq.NewTask(TaskProcessIndex, payload, asynq.MaxRetry(0))
}

func NewLegacyImportTask(jobID, itemID, domainID string) *asynq.Task {
	payload, _ := json.Marshal(LegacyImportPayload{
		JobID:    jobID,
		ItemID:   itemID,
		DomainID: domainID,
	})
	return asynq.NewTask(TaskLegacyImport, payload, asynq.MaxRetry(2))
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

func NewServer(cfg config.Config, concurrency int, includeGenQueues bool, includeLinkQueues bool) *asynq.Server {
	genShards := cfg.GenQueueShards
	if genShards < 1 {
		genShards = 1
	}
	linkShards := cfg.LinkQueueShards
	if linkShards < 1 {
		linkShards = 1
	}
	queues := map[string]int{
		defaultQueue: 1,
	}
	if includeGenQueues && genShards > 1 {
		for i := 0; i < genShards; i++ {
			queues[GenerationQueueName(i)] = 1
		}
	}
	if includeLinkQueues && linkShards > 1 {
		for i := 0; i < linkShards; i++ {
			queues[LinkQueueName(i)] = 1
		}
	}
	return asynq.NewServer(
		asynq.RedisClientOpt{
			Addr:     cfg.RedisAddr,
			Password: cfg.RedisPassword,
			DB:       cfg.RedisDB,
		},
		asynq.Config{
			Concurrency: concurrency,
			Queues:      queues,
		},
	)
}

// NewSchedulerServer returns a server that only consumes scheduler tick tasks.
// This prevents the scheduler process from competing with the worker on the default queue.
func NewSchedulerServer(cfg config.Config, concurrency int) *asynq.Server {
	return asynq.NewServer(
		asynq.RedisClientOpt{
			Addr:     cfg.RedisAddr,
			Password: cfg.RedisPassword,
			DB:       cfg.RedisDB,
		},
		asynq.Config{
			Concurrency: concurrency,
			Queues: map[string]int{
				schedulerQueue: 1,
			},
		},
	)
}

func SchedulerQueueName() string {
	return schedulerQueue
}

// GenerationQueueName возвращает имя очереди генерации по индексу.
func GenerationQueueName(idx int) string {
	return fmt.Sprintf("%s:%d", genQueuePrefix, idx)
}

// LinkQueueName возвращает имя очереди линкбилдинга по индексу.
func LinkQueueName(idx int) string {
	return fmt.Sprintf("%s:%d", linkQueuePrefix, idx)
}

// QueueForProject распределяет проекты по очередям генерации для более справедливой параллельности.
func QueueForProject(projectID string, shards int) string {
	return queueForProject(projectID, shards, genQueuePrefix)
}

// QueueForLinkProject распределяет link-задачи по очередям для более справедливой параллельности.
func QueueForLinkProject(projectID string, shards int) string {
	return queueForProject(projectID, shards, linkQueuePrefix)
}

func queueForProject(projectID string, shards int, prefix string) string {
	if shards <= 1 || strings.TrimSpace(projectID) == "" {
		return defaultQueue
	}
	h := fnv.New32a()
	_, _ = h.Write([]byte(projectID))
	idx := int(h.Sum32() % uint32(shards))
	if prefix == linkQueuePrefix {
		return LinkQueueName(idx)
	}
	return GenerationQueueName(idx)
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

func ParseIndexCheckPayload(task *asynq.Task) (IndexCheckPayload, error) {
	var p IndexCheckPayload
	if err := json.Unmarshal(task.Payload(), &p); err != nil {
		return p, fmt.Errorf("invalid payload: %w", err)
	}
	if strings.TrimSpace(p.CheckID) == "" {
		return p, fmt.Errorf("missing check id")
	}
	return p, nil
}

func ParseLegacyImportPayload(task *asynq.Task) (LegacyImportPayload, error) {
	var p LegacyImportPayload
	if err := json.Unmarshal(task.Payload(), &p); err != nil {
		return p, fmt.Errorf("invalid payload: %w", err)
	}
	if p.JobID == "" || p.ItemID == "" || p.DomainID == "" {
		return p, fmt.Errorf("missing ids")
	}
	return p, nil
}
