# Расписания и Link Tasks: инструкция

Документ описывает работу с расписаниями генерации и задачами по ссылкам (Link Tasks).

## Расписания (Schedules)

### Зачем нужны
Расписания автоматически формируют очередь на генерацию доменов. Запуск может быть регулярным или ручным.

### Как создать через UI
1. Откройте страницу проекта.
2. Перейдите на вкладку `Schedules`.
3. Нажмите `Create schedule`.
4. Заполните `Name`.
5. Выберите `Strategy`.
6. Заполните `Config`.
7. Включите `Active`, если расписание должно работать сразу.
8. Нажмите `Save`.

### Стратегии и конфигурация
- `immediate`: запуск сразу после создания. Конфигурация может быть пустой.
- `daily`: ежедневный запуск.
  - `limit`: сколько доменов поставить в очередь за запуск.
  - `time`: время запуска в формате `HH:MM`.
- `weekly`: еженедельный запуск.
  - `limit`: сколько доменов поставить в очередь за запуск.
  - `day`: день недели.
  - `time`: время запуска в формате `HH:MM`.
- `custom`: произвольный cron.
  - `cron`: строка cron, например `0 9 * * *`.

### Ручной запуск
В списке расписаний нажмите `Trigger Now`. Это сразу формирует задания в очереди.

### Что происходит на стороне сервера
- Планировщик выбирает активные расписания.
- Для подходящих расписаний домены ставятся в `generation_queue`.
- Worker забирает очередь и создает задачи генерации.

### API
```http
POST   /api/projects/:id/schedules
GET    /api/projects/:id/schedules
PATCH  /api/projects/:id/schedules/:scheduleId
DELETE /api/projects/:id/schedules/:scheduleId
POST   /api/projects/:id/schedules/:scheduleId/trigger
```

Пример создания:
```json
{
  "name": "Daily 5",
  "strategy": "daily",
  "config": { "limit": 5, "time": "09:00" },
  "is_active": true
}
```

Пример custom:
```json
{
  "name": "Cron custom",
  "strategy": "custom",
  "config": { "cron": "0 9 * * *" },
  "is_active": true
}
```

## Link Tasks (задачи на ссылки)

### Зачем нужны
Link Tasks вставляют ссылки в HTML на опубликованном сайте.  
Если анкор не найден, система может сгенерировать контент с анкором через LLM.

### Как добавить через UI
1. Откройте страницу домена.
2. Перейдите на вкладку `Links`.
3. Заполните поля:
   - `Anchor Text`
   - `Target URL`
   - `Scheduled For` (необязательно)
4. Нажмите `Save` или `Save & Add Another`.

### CSV импорт
Можно добавить список задач из CSV.

Формат строк:
```
anchor_text,target_url,scheduled_for
```

Пример:
```
anchor_text,target_url,scheduled_for
bonus 1xbet,https://example.com,2026-02-04T10:00:00Z
fast withdraw,https://example.com,
```

Требования:
- `anchor_text` и `target_url` обязательны.
- `target_url` должен начинаться с `http://` или `https://`.
- `scheduled_for` можно не указывать.

### Статусы
- `pending` — ожидает обработки.
- `searching` — поиск места вставки.
- `inserted` — ссылка вставлена.
- `generated` — сгенерирован параграф с анкором.
- `failed` — ошибка.

### Действия
- `Retry` — повторить задачу.
- `Edit` — изменить время запуска.
- `Delete` — удалить задачу.
- `Bulk Retry` — повторить выбранные задачи.
- `Bulk Delete` — удалить выбранные задачи.

### API
```http
POST   /api/domains/:id/links
POST   /api/domains/:id/links/import
GET    /api/domains/:id/links
GET    /api/links
PATCH  /api/links/:id
DELETE /api/links/:id
POST   /api/links/:id/retry
```
