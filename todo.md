Да, это лучше дореализовывать не через editor-agent, а как отдельный bounded step внутри worker pipeline. Иначе перед публикацией у тебя появится медленный и плохо предсказуемый loop.

Сейчас архитектура такая:
- `AssemblyStep` собирает `generated_files`, `final_html`, `zip_archive` в [step_assembly.go](/Users/sukhrobshukurov/Dev/codex-testing/pbn-generator/internal/worker/pipeline/step_assembly.go#L23)
- `AuditStep` только находит проблемы и пишет `audit_report` в [step_audit.go](/Users/sukhrobshukurov/Dev/codex-testing/pbn-generator/internal/worker/pipeline/step_audit.go#L27)
- `PublishStep` вообще не смотрит на `audit_status` и всегда публикует `zip_archive` в [step_publish.go](/Users/sukhrobshukurov/Dev/codex-testing/pbn-generator/internal/worker/pipeline/step_publish.go#L32)
- порядок шагов зашит в [process_generation.go](/Users/sukhrobshukurov/Dev/codex-testing/pbn-generator/cmd/worker/process_generation.go#L387)

Я бы делал так.

**Правильная форма**
1. Оставить `AuditStep` как детектор, но вынести его логику в чистый helper `runAudit(files)`.
2. Добавить новый шаг `AuditFixStep` между аудитом и публикацией.
3. Не запускать внутри него агентный loop. Делать один bounded LLM pass на файл, максимум 1 corrective retry по паттерну, который у вас уже есть в [brand_guard.go](/Users/sukhrobshukurov/Dev/codex-testing/pbn-generator/internal/worker/pipeline/brand_guard.go#L108).
4. После фикса внутри этого же шага пересчитать аудит заново и обновить артефакты.
5. Публикацию делать только после финального состояния, а не после “первого аудита”.

**Почему именно так**
- В worker уже есть `LLMClient.Generate(...)` и `PromptManager` в [types.go](/Users/sukhrobshukurov/Dev/codex-testing/pbn-generator/internal/worker/pipeline/types.go#L10), значит отдельный editor-agent не нужен.
- Pipeline пропускает шаг по `ArtifactKey()` в [pipeline.go](/Users/sukhrobshukurov/Dev/codex-testing/pbn-generator/internal/worker/pipeline/pipeline.go#L149). Поэтому просто вставить второй `AuditStep` нельзя: он будет конфликтовать по `audit_report`. Надо либо переиспользовать audit helper внутри `AuditFixStep`, либо вводить отдельные ключи.
- `PublishStep` публикует именно `zip_archive`, а не `generated_files` в [step_publish.go](/Users/sukhrobshukurov/Dev/codex-testing/pbn-generator/internal/worker/pipeline/step_publish.go#L42). Значит после любых автофиксов надо обязательно пересобрать `zip_archive`, иначе на сервер уйдет старая сборка.

**Безопасный план внедрения**
1. Фаза 1: scaffolding без изменения поведения.
- Вынести audit-движок из [step_audit.go](/Users/sukhrobshukurov/Dev/codex-testing/pbn-generator/internal/worker/pipeline/step_audit.go#L27) в helper.
- Расширить finding модель полями типа `blocking`, `autofixable`, `target_files`, `fix_kind`.
- Добавить `AuditFixStep`, но в режиме `disabled/report_only`.
- По умолчанию генерация должна работать как сейчас.

2. Фаза 2: мягкий autofix только для allowlist.
- Новый шаг `AuditFixStep` читает `generated_files` и `audit_report`.
- Берет только безопасные, однофайловые, текстовые проблемы.
- Делает один LLM-вызов на файл в формате “вот файл + вот findings + верни полный исправленный контент JSON”.
- Максимум 1 corrective retry, как в `brand_guard`.
- Никаких multi-file правок, никаких бинарников, никаких image rewrites.

3. Фаза 3: пересборка артефактов после фикса.
- После изменения `generated_files` шаг должен:
  - обновить `generated_files`
  - если менялся `index.html`, обновить `final_html`
  - если менялся `404.html`, обновить `404_html`
  - заново собрать `zip_archive`
- Это критично, иначе `PublishStep` отправит старый архив.

4. Фаза 4: повторный аудит и gating.
- В конце `AuditFixStep` снова прогнать `runAudit(files)`.
- Сохранить:
  - `audit_report_before_fix`
  - `audit_report`
  - `audit_fix_result`
  - `audit_fix_applied`
- Потом уже решать, можно ли публиковать.

5. Фаза 5: режимы rollout.
- `report_only` — текущее поведение, default.
- `autofix_soft` — пробуем чинить, но если не получилось, всё равно публикуем и логируем.
- `autofix_strict` — если после фикса остались blocking issues, до `PublishStep` не доходим.
- Начинать только с `report_only` и `autofix_soft`.

**Что чинить в первой версии**
Не всё подряд. Только whitelist.
- `missing_required_file`
- `missing_asset` только для текстовых HTML/CSS кейсов, где правка локальна
- потом уже SEO/markup rules вроде `missing title`, `missing h1`, `broken internal link`, если добавите их в аудит

С текущими правилами ROI ограничен: audit пока слишком узкий. Поэтому параллельно с scaffolding я бы расширял сами machine-readable rules.

**Что не делать в первой версии**
- Не тащить editor-agent внутрь worker.
- Не делать tool loop до публикации.
- Не давать модели менять несколько файлов за раз.
- Не блокировать publish глобально с первого дня.

**Какие файлы менять**
- [step_audit.go](/Users/sukhrobshukurov/Dev/codex-testing/pbn-generator/internal/worker/pipeline/step_audit.go)
- новый `step_audit_fix.go` рядом в `internal/worker/pipeline`
- [step_publish.go](/Users/sukhrobshukurov/Dev/codex-testing/pbn-generator/internal/worker/pipeline/step_publish.go)
- [process_generation.go](/Users/sukhrobshukurov/Dev/codex-testing/pbn-generator/cmd/worker/process_generation.go#L387)
- тесты по pipeline и шагам

**Какой порядок я бы взял**
1. Вынести `runAudit(files)` и не менять поведение.
2. Добавить `AuditFixStep` в disabled mode.
3. Научить его чинить 1-2 безопасных rule codes.
4. Пересобирать `zip_archive` после правок.
5. Включить `autofix_soft` только на тестовых доменах.
6. Лишь потом добавить `autofix_strict`.

Если хочешь, следующим сообщением я могу уже собрать тебе конкретный implementation roadmap по PR-ам: `какие новые step'ы`, `какие artifact keys`, `какие тесты`, `какие feature flags`.