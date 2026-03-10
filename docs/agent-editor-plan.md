# AI Agent Editor — План реализации

> Дата создания: 2026-03-10
> Статус: В планировании
> Ветка: `feature/agent-editor`

---

## Суть задачи

Заменить текущий **EditorAIStudioPanel** (one-shot форма, 1094 строки) на полноценный **AI-агент в виде чата** — аналог Cursor/Claude Code внутри редактора сайтов.

**Текущий AI Studio делает:**
- Редактирует открытый файл → медленно, контекст формируется вручную
- Создаёт страницы → непредсказуемый результат
- Генерирует картинки → сохраняется как инструмент агента

**Агент делает:**
- Сам читает нужные файлы
- Сам решает что и как менять
- Сам генерирует картинки если нужно
- Мультиходовой диалог (follow-up сообщения)
- История + откат любой сессии

---

## Архитектура Layout

### До

```
┌────────────────────────────────────────────────────┐
│  Header                                            │
├──────────────┬─────────────────────────────────────┤
│              │  EditorFileWorkspacePanel            │
│  Sidebar     │  (Monaco Editor / Preview)           │
│  (320px)     ├─────────────────────────────────────┤
│  File tree   │  EditorAIStudioPanel (УДАЛЯЕТСЯ)     │
│              ├─────────────────────────────────────┤
│              │  EditorHistoryAccess                 │
└──────────────┴─────────────────────────────────────┘
```

### После

```
┌─────────────┬──────────────────────────┬────────────────┐
│             │  EditorFileWorkspace     │                │
│  Sidebar    │  Panel                   │  AgentChat     │
│  (320px)    │  (Monaco / Preview)      │  Panel         │
│  File tree  │                          │  (380px)       │
│             │  [больше места]          │  [toggleable]  │
│             │                          │                │
│             │  EditorHistoryAccess     │                │
└─────────────┴──────────────────────────┴────────────────┘
```

Кнопка **`[✦ AI Агент]`** в header — показывает/скрывает правую панель.

---

## Стек технологий

- **Backend AI:** Anthropic Claude API (`claude-sonnet-4-6`)
- **Изображения:** Gemini API (`gemini-2.5-flash-image`) — существующий код
- **Streaming:** SSE (Server-Sent Events)
- **Go SDK:** `github.com/anthropics/anthropic-sdk-go`

---

## Backend — детали реализации

### Config (`pbn-generator/internal/config/config.go`)

```go
// Добавить в struct Config:
AnthropicAPIKey  string  // env: ANTHROPIC_API_KEY
AnthropicModel   string  // env: ANTHROPIC_MODEL, default: claude-sonnet-4-6
AgentMaxTokens   int     // env: AGENT_MAX_TOKENS, default: 8192
AgentTimeoutSec  int     // env: AGENT_TIMEOUT_SEC, default: 180
```

### `.env.example`

```
ANTHROPIC_API_KEY=sk-ant-api03-...
ANTHROPIC_MODEL=claude-sonnet-4-6
AGENT_MAX_TOKENS=8192
AGENT_TIMEOUT_SEC=180
```

### Новые файлы

```
pbn-generator/internal/httpserver/
  agent_handlers.go     — SSE endpoint + агентный цикл
  agent_tools.go        — определение и исполнение инструментов
  agent_snapshot.go     — снэпшот/rollback через revision history
```

### Маршруты (`routes.go`)

```go
// Под authz домена (editor/manager role):
POST   /api/domains/{id}/agent                        — запуск/продолжение сессии (SSE)
POST   /api/domains/{id}/agent/{session_id}/stop      — остановить агента
POST   /api/domains/{id}/agent/{session_id}/rollback  — откатить все изменения сессии
GET    /api/domains/{id}/agent/sessions               — история сессий домена
GET    /api/domains/{id}/agent/sessions/{session_id}  — детали сессии
```

### Инструменты агента (`agent_tools.go`)

```go
var domainAgentTools = []anthropic.Tool{
    {
        Name:        "list_files",
        Description: "Список всех файлов сайта с типами и размерами. Используй чтобы понять структуру сайта.",
        InputSchema: /* { directory?: string } */
    },
    {
        Name:        "read_file",
        Description: "Прочитать содержимое файла. Всегда читай файл перед его изменением.",
        InputSchema: /* { path: string } */
    },
    {
        Name:        "write_file",
        Description: "Создать или обновить файл. Разрешены типы: html, css, js, txt, json, xml, svg.",
        InputSchema: /* { path: string, content: string } */
    },
    {
        Name:        "delete_file",
        Description: "Удалить файл. Нельзя удалять index.html и корневые конфиги.",
        InputSchema: /* { path: string } */
    },
    {
        Name:        "generate_image",
        Description: "Сгенерировать изображение через Gemini и сохранить в директории assets/. Путь должен начинаться с 'assets/'.",
        InputSchema: /* { path: string, prompt: string, alt_text: string } */
    },
    {
        Name:        "search_in_files",
        Description: "Найти текст по всем файлам сайта. Полезно для поиска конкретных элементов.",
        InputSchema: /* { query: string } */
    },
}
```

**Исполнитель `generate_image`** — переиспользует существующий `generateEditorImage()` из `editor_ai_service_helpers.go` без изменений.

**Валидация инструментов:**
```go
// Запрещено агенту:
// - Писать бинарные файлы (только текстовые расширения)
// - Удалять index.html
// - Создавать файлы вне корня домена (path traversal)
// - Писать файлы > 2MB
// - Генерировать изображения вне assets/
```

### Агентный цикл (`agent_handlers.go`)

```go
func (s *Server) handleDomainAgent(w http.ResponseWriter, r *http.Request) {
    // 1. Auth + authz
    // 2. Parse request: { message, session_id? }
    // 3. Если новая сессия — создать снэпшот всех файлов
    // 4. Загрузить историю сообщений если session_id передан
    // 5. Настроить SSE
    // 6. Запустить агентный цикл с timeout

    ctx, cancel := context.WithTimeout(r.Context(), agentTimeout)
    defer cancel()

    messages := buildMessages(history, userMessage, domain)

    for iteration := 0; iteration < maxIterations; iteration++ {
        response, err := anthropicClient.Messages.NewStreaming(ctx, params)
        // Стримить текст → SSE type:text
        // Собрать tool_use блоки

        if response.StopReason == "end_turn" {
            // Сохранить итог сессии
            sseEvent(w, "done", summary)
            break
        }

        // Исполнить tool calls
        toolResults := []anthropic.ToolResultBlockParam{}
        for _, toolUse := range response.ToolUses {
            sseEvent(w, "tool_start", toolUse)
            result := s.executeAgentTool(ctx, domain, toolUse)
            sseEvent(w, "tool_done", result)
            toolResults = append(toolResults, result)

            // Если write_file/generate_image — уведомить frontend
            if toolUse.Name == "write_file" || toolUse.Name == "generate_image" {
                sseEvent(w, "file_changed", changedFile)
            }
        }

        messages = append(messages, assistantMsg, toolResultsMsg)
    }
}
```

### SSE события (полный список)

```
// Начало сессии
{"type":"session_start","session_id":"abc","snapshot_id":"snap123","is_new":true}

// Стриминг текста от агента
{"type":"text","delta":"Анализирую структуру сайта..."}

// Начало вызова инструмента
{"type":"tool_start","id":"t1","tool":"read_file","input":{"path":"index.html"}}

// Завершение вызова инструмента (с превью содержимого)
{"type":"tool_done","id":"t1","tool":"read_file","preview":"<!DOCTYPE html>...","truncated":false}

// Файл изменён (для обновления в Monaco + sidebar)
{"type":"file_changed","path":"about.html","action":"created|updated|deleted"}

// Изображение сгенерировано
{"type":"image_generated","path":"assets/hero.webp","url":"/api/domains/.../files/assets/hero.webp?raw=1"}

// Ошибка инструмента
{"type":"tool_error","id":"t2","tool":"write_file","error":"file too large"}

// Агент остановлен пользователем
{"type":"stopped","message":"Остановлено по запросу"}

// Успешное завершение
{"type":"done","summary":"Создано 2 файла, изменён 1 файл","files_changed":["about.html","assets/hero.webp"]}

// Ошибка агента
{"type":"error","message":"Anthropic API недоступен","rollback_available":true}
```

### Системный промпт агента

```
Ты — AI-ассистент редактора сайтов. Тебе дан доступ к файлам сайта {{ domain_url }}.

Информация о сайте:
- URL: {{ domain_url }}
- Язык: {{ language }}
- Ключевая тема: {{ keyword }}
- Тип генерации: {{ generation_type }}

ПРАВИЛА:
1. ВСЕГДА сначала прочитай файлы которые планируешь менять (read_file)
2. Используй list_files чтобы понять структуру перед созданием новых файлов
3. Сохраняй существующий стиль, тему и язык сайта
4. HTML должен быть валидным
5. Изображения всегда сохраняй в папку assets/
6. Не создавай дублирующих файлов
7. При создании изображений — пиши детальный prompt на английском для лучшего результата
8. Отвечай на языке пользователя: {{ user_language }}
9. Если задача непонятна — уточни перед началом работы
10. Сообщай что делаешь в процессе работы
```

### Снэпшот и Rollback (`agent_snapshot.go`)

```go
// Снэпшот — пакетный CreateRevision для всех файлов домена
func (s *Server) createAgentSnapshot(ctx context.Context, domain, sessionID string) error {
    files, _ := s.siteFiles.ListByDomain(ctx, domain.ID)
    for _, file := range files {
        content, _ := s.readDomainFileBytesFromBackend(ctx, domain, file.Path)
        s.fileEdits.CreateRevision(ctx, Revision{
            FileID:    file.ID,
            Content:   content,
            Tag:       "agent_snapshot",
            SessionID: sessionID,
            CreatedBy: "agent",
        })
    }
}

// Rollback — восстановить все файлы из снэпшота сессии
func (s *Server) rollbackAgentSession(ctx context.Context, domain, sessionID string) (int, error) {
    revisions, _ := s.fileEdits.ListBySessionTag(ctx, sessionID, "agent_snapshot")
    restored := 0
    for _, rev := range revisions {
        s.writeDomainFileBytesToBackend(ctx, domain, rev.Path, rev.Content)
        restored++
    }
    return restored, nil
}
```

### История сессий (DB)

```sql
-- Новая таблица
CREATE TABLE agent_sessions (
    id          TEXT PRIMARY KEY,
    domain_id   TEXT NOT NULL REFERENCES domains(id),
    created_by  TEXT NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    finished_at TIMESTAMPTZ,
    status      TEXT NOT NULL DEFAULT 'running', -- running|done|error|stopped|rolled_back
    summary     TEXT,
    files_changed JSONB,   -- ["about.html", "assets/hero.webp"]
    message_count INT DEFAULT 0,
    snapshot_id TEXT       -- для rollback
);
```

---

## Frontend — детали реализации

### Новые файлы

```
frontend/features/editor-v3/
  components/
    AgentChatPanel.tsx          — основная панель чата
    AgentMessage.tsx            — сообщение пользователя / агента (с markdown)
    AgentToolCallEvent.tsx      — визуализация tool call (раскрывающийся)
    AgentImagePreview.tsx       — превью сгенерированного изображения inline
    AgentSessionRollback.tsx    — кнопка отката с диалогом подтверждения
    AgentSessionHistory.tsx     — список прошлых сессий
    AgentSuggestedPrompts.tsx   — быстрые подсказки при пустом чате
  hooks/
    useAgentSession.ts          — SSE клиент + session state machine
    useAgentHistory.ts          — загрузка истории сессий
  types/
    agent.ts                    — типы SSE событий, сессий, сообщений
```

### Удаляемые файлы

```
frontend/features/editor-v3/components/EditorAIStudioPanel.tsx  ← УДАЛИТЬ
frontend/features/editor-v3/hooks/useAIFlowState.ts             ← УДАЛИТЬ (если не используется в другом месте)
frontend/features/editor-v3/types/ai.ts                         ← УДАЛИТЬ или переработать
```

### UI чата — полное описание

```
┌─────────────────────────────────────────┐
│  ✦ AI Агент              [История] [×]  │
├─────────────────────────────────────────┤
│                                         │
│  [Пустое состояние — быстрые промпты]:  │
│  ┌─────────────┐  ┌─────────────┐       │
│  │ Улучши SEO  │  │ Добавь /    │       │
│  │             │  │ about       │       │
│  └─────────────┘  └─────────────┘       │
│  ┌─────────────┐  ┌─────────────┐       │
│  │ Исправь     │  │ Мобильная   │       │
│  │ стили       │  │ версия      │       │
│  └─────────────┘  └─────────────┘       │
│                                         │
│  ─── или после сообщений ───            │
│                                         │
│  ┌─────────────────────────────────┐    │
│  │ 👤 Добавь страницу /about       │    │
│  │    с разделом команды           │    │
│  └─────────────────────────────────┘    │
│                                         │
│  ✦ Анализирую структуру сайта...        │
│                                         │
│  ▶ list_files()                         │
│    5 файлов: index.html, style.css, ... │
│                                         │
│  ▶ read_file("index.html")  [раскрыть ▼]│
│  ▶ read_file("style.css")   [раскрыть ▼]│
│                                         │
│  Создаю about.html в стиле главной      │
│  страницы с тремя карточками команды.   │
│  Сгенерирую 3 изображения для фото.     │
│                                         │
│  ▶ generate_image("assets/person1.webp")│
│    ┌────────────────────────────────┐   │
│    │    [превью изображения]        │   │
│    └────────────────────────────────┘   │
│    ✅ Сохранено                         │
│                                         │
│  ▶ generate_image("assets/person2.webp")│
│    ✅ Сохранено [показать превью]       │
│                                         │
│  ▶ write_file("about.html")             │
│    + 87 строк добавлено      [открыть] │
│    ✅ Создан                            │
│                                         │
│  Готово! Создана страница /about:       │
│  • Секция команды с 3 фото             │
│  • Навигация обновлена в index.html    │
│  • 3 изображения сгенерированы         │
│                                         │
│  [↩ Откатить эту сессию]               │
│                                         │
├─────────────────────────────────────────┤
│ [ ] Включить текущий файл в контекст   │
│                                         │
│  ┌─────────────────────────────────┐    │
│  │ Написать агенту...              │    │
│  └─────────────────────────────────┘    │
│  [■ Стоп]              [Отправить ↵]   │
└─────────────────────────────────────────┘
```

### Поведение при изменении файлов

```typescript
// В useAgentSession.ts при получении события file_changed:
if (event.path === currentOpenFile) {
    reloadFileContent()        // Автообновление Monaco без перезагрузки
    showToast("Агент обновил текущий файл")
}
markFileAsChanged(event.path) // Значок ✦ в sidebar дереве файлов
```

### Keyboard shortcuts

| Комбинация | Действие |
|-----------|---------|
| `Cmd+Enter` | Отправить сообщение |
| `Escape` | Остановить агента (во время работы) |
| `Cmd+Shift+A` | Показать/скрыть панель агента |

### Быстрые подсказки (AgentSuggestedPrompts)

```typescript
const SUGGESTED_PROMPTS = [
    "Улучши SEO: мета-теги, заголовки, alt-тексты",
    "Добавь страницу /about с описанием сайта",
    "Исправь мобильную вёрстку (responsive)",
    "Обнови стили: сделай дизайн современнее",
    "Добавь страницу /contact с формой",
    "Проверь и исправь все битые внутренние ссылки",
    "Оптимизируй скорость: минифицируй CSS и JS",
    "Добавь favicon и Open Graph мета-теги",
]
```

### Передача контекста текущего файла

```typescript
// В запросе агенту:
{
    message: "...",
    session_id?: "...",
    context: {
        current_file: "about.html",   // текущий открытый файл
        include_current_file: true,   // чекбокс в UI
    }
}
```

### История сессий (AgentSessionHistory)

Слайдер сверху или модальное окно:
```
┌─────────────────────────────────────────┐
│ История сессий агента                   │
├─────────────────────────────────────────┤
│ Сегодня, 09:00 — [done]                 │
│ Создана страница /about, 3 изображения  │
│ Изменено файлов: 4         [Откатить]   │
├─────────────────────────────────────────┤
│ Вчера, 14:32 — [done]                   │
│ Обновлены мета-теги SEO                 │
│ Изменено файлов: 1         [Откатить]   │
├─────────────────────────────────────────┤
│ Вчера, 11:15 — [rolled_back]            │
│ Попытка редизайна (откачено)            │
│ Изменено файлов: 0         [—]          │
└─────────────────────────────────────────┘
```

---

## Безопасность

### Ограничения инструментов

| Проверка | Реализация |
|----------|-----------|
| Path traversal | Запрет `../`, абсолютных путей, символических ссылок |
| Тип файла | Whitelist: html, css, js, ts, json, xml, svg, txt, md |
| Размер файла | Max 2MB на запись |
| Изображения только в assets/ | Проверка prefix пути |
| Нельзя удалить index.html | Явная проверка |
| Только файлы своего домена | domainID в каждом вызове инструмента |

### Снэпшот

- Создаётся перед первым вызовом инструмента в новой сессии
- Хранится как пакет ревизий с тегом `agent_session:{id}`
- Rollback восстанавливает все файлы из снэпшота
- Если файл был создан агентом (не существовал) — при rollback удаляется

### При сбоях

| Сбой | Поведение |
|------|---------|
| SSE разрыв (браузер закрыт) | `context.Done()` → агент останавливается |
| Anthropic API timeout | Ошибка, снэпшот доступен для ручного rollback |
| Gemini API ошибка при generate_image | Tool возвращает ошибку, агент продолжает без картинки |
| Ошибка записи файла | Tool error → агент сообщает пользователю |
| Агент превысил max_iterations | Принудительная остановка, показать что успело сделать |

---

## Оценка стоимости

Модель: `claude-sonnet-4-6` — $3/MTok вход, $15/MTok выход

| Тип задачи | Входящих токенов | Исходящих | Стоимость |
|-----------|-----------------|-----------|-----------|
| Лёгкая (3 файла) | ~10K | ~1.5K | ~$0.05 |
| Средняя (7 файлов) | ~25K | ~3K | ~$0.12 |
| Сложная (15 файлов + картинки) | ~60K | ~6K | ~$0.27 |
| **50 сессий/день** | | | **~$90-180/мес** |
| **200 сессий/день** | | | **~$360-720/мес** |

Изображения: Gemini `gemini-2.5-flash-image` — по существующему тарифу.

---

## Порядок реализации

### Этап 1 — Backend (3-4 дня)
- [x] `ANTHROPIC_API_KEY` в config + `.env.example`
- [x] `go get github.com/anthropics/anthropic-sdk-go`
- [x] `agent_tools.go` — определения + executor (включая generate_image)
- [x] `agent_snapshot.go` — создание снэпшота + rollback
- [x] `agent_handlers.go` — SSE endpoint + агентный цикл
- [x] `agent_sessions` таблица в БД
- [x] Routes в `routes.go`
- [x] `go build ./...` + `go test ./...`

### Этап 2 — Frontend базовый чат (2-3 дня)
- [x] `agent.ts` — типы SSE событий
- [x] `useAgentSession.ts` — SSE клиент + state
- [x] `AgentChatPanel.tsx` — layout, input, stop button
- [x] `AgentMessage.tsx` — рендеринг markdown
- [x] `AgentToolCallEvent.tsx` — раскрывающиеся события
- [x] Кнопка `[✦ AI Агент]` в header редактора
- [x] Интеграция с EditorContext (авто-обновление Monaco)
- [x] Индикаторы изменённых файлов в sidebar

### Этап 3 — UX полировка (2 дня)
- [x] `AgentImagePreview.tsx` — превью картинок inline
- [x] `AgentSuggestedPrompts.tsx` — быстрые подсказки
- [x] `AgentSessionRollback.tsx` — диалог отката
- [x] `AgentSessionHistory.tsx` — история сессий
- [x] Keyboard shortcuts (Alt+A открыть/закрыть, Escape закрыть)
- [x] Удаление `EditorAIStudioPanel` из editor page

### Этап 4 — Тестирование
- [ ] Тест: агент читает файлы корректно
- [ ] Тест: агент пишет файлы, Monaco обновляется
- [ ] Тест: generate_image работает через инструмент
- [ ] Тест: SSE разрыв → корректная остановка
- [ ] Тест: rollback восстанавливает все файлы
- [ ] Тест: path traversal атаки блокируются
- [ ] Нагрузочный тест: 10 параллельных сессий

---

## Файлы которые затрагивает реализация

### Backend — новые файлы
- `pbn-generator/internal/httpserver/agent_handlers.go`
- `pbn-generator/internal/httpserver/agent_tools.go`
- `pbn-generator/internal/httpserver/agent_snapshot.go`
- `pbn-generator/internal/db/migrations/YYYYMMDD_agent_sessions.sql`

### Backend — изменяемые файлы
- `pbn-generator/internal/config/config.go` — добавить AnthropicAPIKey и др.
- `pbn-generator/internal/httpserver/routes.go` — добавить agent routes
- `pbn-generator/internal/httpserver/server_types.go` — добавить AgentSessionStore
- `pbn-generator/go.mod` — добавить anthropic-sdk-go
- `.env.example` — добавить ANTHROPIC_API_KEY

### Frontend — новые файлы
- `frontend/features/editor-v3/components/AgentChatPanel.tsx`
- `frontend/features/editor-v3/components/AgentMessage.tsx`
- `frontend/features/editor-v3/components/AgentToolCallEvent.tsx`
- `frontend/features/editor-v3/components/AgentImagePreview.tsx`
- `frontend/features/editor-v3/components/AgentSessionRollback.tsx`
- `frontend/features/editor-v3/components/AgentSessionHistory.tsx`
- `frontend/features/editor-v3/components/AgentSuggestedPrompts.tsx`
- `frontend/features/editor-v3/hooks/useAgentSession.ts`
- `frontend/features/editor-v3/hooks/useAgentHistory.ts`
- `frontend/features/editor-v3/types/agent.ts`

### Frontend — изменяемые файлы
- `frontend/app/(app)/domains/[id]/editor/page.tsx` — убрать AI Studio, добавить Agent
- `frontend/features/editor-v3/context/EditorContext.tsx` — добавить agent file change callbacks
- `frontend/features/editor-v3/components/EditorSidebar.tsx` — добавить индикаторы изменённых файлов

### Frontend — удаляемые файлы
- `frontend/features/editor-v3/components/EditorAIStudioPanel.tsx`
- `frontend/features/editor-v3/hooks/useAIFlowState.ts` *(проверить использование)*
- `frontend/features/editor-v3/types/ai.ts` *(проверить использование)*

---

## Ключевые решения

| Вопрос | Решение |
|--------|---------|
| AI Studio сохранить? | Нет — полностью заменяется агентом |
| Какая модель? | claude-sonnet-4-6 (оптимальное соотношение цена/качество) |
| Картинки через агента? | Да — инструмент generate_image вызывает существующий generateEditorImage() |
| Откат автоматический при ошибке? | Нет — пользователь сам нажимает откат (могут быть частичные изменения которые нужны) |
| История хранится где? | В таблице agent_sessions + revision history |
| Максимум итераций агента? | 20 (защита от бесконечного цикла) |
| Таймаут сессии? | 3 минуты (AGENT_TIMEOUT_SEC) |
| Мультиходовой диалог? | Да — session_id передаётся в повторных запросах |
