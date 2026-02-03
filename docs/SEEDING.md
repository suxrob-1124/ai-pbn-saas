# Сидинг через backend API (dev)

Этот проект использует bash‑скрипт `scripts/seed_backend.sh` для создания тестовых данных через API.
Скрипт идемпотентен и безопасен для повторного запуска.

## Предварительные условия

- Запущен backend (`http://localhost:8080`)
- В `.env` включены `BOOTSTRAP_ADMIN_EMAIL=admin@example.com` и `AUTO_APPROVE_USERS=true`

## Запуск

```bash
./scripts/seed_backend.sh
```

## Что создаётся

- Пользователи: `admin@example.com` / `Admin123!!`
- Пользователи: `manager@example.com` / `Manager123!!`
- Пользователи: `manager2@example.com` / `Manager123!!`
- Пользователи: `user@example.com` / `User123!!!`
- Проекты: `surstrem` (manager)
- Проекты: `1xbet-ru` (manager2)
- Домены: `surstrem` → `profitnesscamps.se`, `elinloe.se`, `kundservice.net`
- Домены: `1xbet-ru` → `скважина61.рф`, `dialog-c.ru`, `autogornostay.ru`

## Примечания

- Скрипт сначала пытается логин, затем регистрирует пользователя (если нужно).
- Если пользователь существует с неизвестным паролем, скрипт сбрасывает пароль через админ‑endpoint.
