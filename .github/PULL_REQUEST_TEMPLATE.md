## Summary

- Что изменено:
- Почему:

## Verification

- [ ] `cd frontend && npm run -s verify:openapi-lint`
- [ ] `cd frontend && npm run -s verify:docs-links`
- [ ] `cd frontend && npm run -s verify:openapi-route-coverage`

## Docs/OpenAPI Policy

- [ ] Если изменены backend API routes/DTO/contracts, обновлен `frontend/openapi.yaml`.
- [ ] Если изменены пользовательские потоки/CTA/статусы, обновлены docs (`frontend/docs-content/*`).
- [ ] Если добавлен новый docs route, обновлены навигация и overview (`DocsSidebar` и `/docs`).
