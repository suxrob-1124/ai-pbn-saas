import type { DocsPageContent } from "./types";

export const docsPages = {
  projects: {
    title: "Проекты",
    description: "Настройка проектов и общих параметров в SiteGen AI.",
    intro: "Проект объединяет домены, расписания и общие параметры (страна, язык, таймзона).",
    sections: [
      {
        paragraphs: [
          "При создании проекта укажите название, целевую страну и язык. Таймзона проекта используется для отображения времени и расчёта следующих запусков.",
          "API-ключ берётся у владельца проекта. Если у владельца нет ключа, генерация не будет запущена.",
          "В проекте доступен вход в мониторинг индексации по проекту — это удобно, когда нужно оценить статус доменов сразу по всей группе.",
        ],
      },
      {
        title: "Рекомендации",
        listType: "bullet",
        listItems: [
          "Выбирайте таймзону проекта сразу и используйте её в расписаниях.",
          "Держите название проекта коротким — оно отображается в списках.",
        ],
      },
    ],
  } satisfies DocsPageContent,

  domains: {
    title: "Домены",
    description: "Добавление доменов и настройка параметров домена.",
    intro: "Домены — это единицы генерации сайтов внутри проекта.",
    sections: [
      {
        paragraphs: [
          "Для каждого домена укажите ключевое слово. Оно используется при SERP-анализе и генерации контента.",
          "В настройках домена находятся поля для ссылок: анкор и акцептор. Это позволяет запускать задачи вставки ссылок без отдельной вкладки.",
          "В карточке домена доступна ссылка на мониторинг индексации, чтобы быстро проверить статус проверок по конкретному домену.",
        ],
      },
      {
        title: "Полезно знать",
        listType: "bullet",
        listItems: [
          "Ссылки вставляются только в body и не затрагивают заголовки.",
          "Если домен перегенерирован, ссылка будет добавлена повторно.",
        ],
      },
    ],
  } satisfies DocsPageContent,

  queue: {
    title: "Очередь",
    description: "Как работает очередь генерации.",
    intro: "Очередь показывает домены, запланированные к запуску.",
    sections: [
      {
        paragraphs: [
          "Если подходящих доменов нет, очередь пустая. Время запуска отображается в таймзоне проекта.",
          "Очистка очереди доступна через кнопку «Очистить». Она удаляет устаревшие или неактуальные элементы.",
        ],
      },
      {
        title: "Важные статусы",
        listType: "bullet",
        listItems: [
          "pending — ожидает постановки в обработку",
          "queued — уже поставлено в очередь воркера",
          "completed/failed — будут удалены при очистке",
        ],
      },
    ],
  } satisfies DocsPageContent,

  schedules: {
    title: "Расписания",
    description: "Настройка расписаний генерации и ссылок.",
    intro: "В проекте есть два типа расписаний: генерация сайтов и вставка ссылок.",
    sections: [
      {
        paragraphs: [
          "Для каждого проекта поддерживается одно расписание генерации и одно расписание ссылок. Ручной запуск не блокирует автоматические запуски.",
          "Поле limit ограничивает количество доменов за один запуск. Время всегда интерпретируется в таймзоне проекта.",
        ],
      },
      {
        title: "Примеры настроек",
        listType: "bullet",
        listItems: [
          "Ежедневно, limit 2, время 09:30.",
          "Еженедельно, limit 10, день недели: суббота.",
        ],
      },
    ],
  } satisfies DocsPageContent,

  links: {
    title: "Ссылки",
    description: "Правила вставки и обновления ссылок.",
    intro: "Вставка ссылок происходит по анкору и акцептору, заданным в настройках домена.",
    sections: [
      {
        paragraphs: [
          "Ссылка добавляется только в body. Заголовки и title исключаются из поиска. Если ссылка уже есть, система пытается заменить её по данным предыдущей задачи.",
          "В «Логах ссылок» можно увидеть статус и дифф вставки. Отдельная задача создаётся на каждый домен.",
        ],
      },
      {
        title: "Статусы задач",
        listType: "bullet",
        listItems: [
          "pending — ожидает выполнения",
          "searching — поиск места вставки",
          "inserted — ссылка вставлена",
          "generated — вставлен сгенерированный контент",
          "failed — ошибка обработки",
        ],
      },
    ],
  } satisfies DocsPageContent,

  errors: {
    title: "Ошибки",
    description: "Как читать ошибки и восстанавливать задачи.",
    intro: "Ошибки генерации и ссылок отображаются в логах задач и на странице проекта.",
    sections: [
      {
        paragraphs: [
          "В случае сетевых ошибок (например, SERP timeout) система повторяет попытки автоматически. Максимум повторов настраивается в бэкенде.",
          "После устранения причины ошибки задачу можно перезапустить вручную через карточку домена или лог ссылки.",
        ],
      },
      {
        title: "Где смотреть",
        listType: "bullet",
        listItems: [
          "Страница проекта → вкладка «Ошибки»",
          "Карточка домена → «Логи ссылок»",
          "Swagger API → endpoints генераций и link tasks",
        ],
      },
    ],
  } satisfies DocsPageContent,

  indexing: {
    title: "Мониторинг · Индексация",
    description: "Мониторинг индексации доменов и аналитика по проверкам.",
    intro:
      "Отслеживание индексации доменов: ежедневные проверки, история попыток, статистика и календарь.",
    sections: [
      {
        paragraphs: [
          "Для каждого домена создаётся ежедневная проверка. Результат фиксируется как «в индексе» или «не в индексе», а ошибки переводят задачу в ретраи.",
          "Обычным пользователям мониторинг доступен из карточки проекта через кнопку «Индексация». Администраторы дополнительно могут открыть глобальный мониторинг через Monitoring → Indexing.",
        ],
      },
      {
        title: "Как работает проверка",
        listType: "numbered",
        listItems: [
          "Сервис indexchecker запускается по cron @every 1h и выполняет тик.",
          "Для каждого опубликованного домена создаётся проверка на текущую дату. Уникальность: пара (domain_id, check_date).",
          "В работу берутся pending/checking проверки с наступившим next_retry_at.",
          "Для каждой проверки вызывается SERP API с запросом site:<domain> (с punycode-нормализацией).",
          "При успешном ответе статус становится success, а is_indexed = true/false.",
          "При ошибке выполняется ретрай или статус переходит в failed_investigation.",
        ],
      },
      {
        title: "Статусы",
        listType: "bullet",
        listItems: [
          "pending — создана, ожидает запуска",
          "checking — идёт проверка или запланирован повтор",
          "success — получен финальный ответ (в индексе/не в индексе)",
          "failed_investigation — превышен лимит попыток",
        ],
        paragraphs: [
          "Важно: «не в индексе» — это корректный результат и считается успешной проверкой.",
        ],
      },
      {
        title: "Какие домены участвуют",
        listType: "bullet",
        listItems: [
          "Только опубликованные домены: status = published или заполнен published_at.",
          "URL домена должен быть задан.",
        ],
        paragraphs: [
          "Для непубликованных доменов проверка и ручной запуск блокируются.",
        ],
      },
      {
        title: "Алгоритм запроса (SERP)",
        listType: "bullet",
        listItems: [
          "Запрос: site:<domain> (punycode).",
          "Гео берётся из домена, иначе из проекта, иначе fallback se.",
          "Таймауты и ретраи SERP используют общие настройки анализатора.",
          "Файлы с диска не читаются: используется только URL и SERP-ответ.",
        ],
      },
      {
        title: "Ретраи и расписание",
        listType: "bullet",
        listItems: [
          "Attempt 1: 30 минут",
          "Attempt 2: 1 час",
          "Attempt 3: 2 часа",
          "Attempt 4: 4 часа",
          "Максимум: 8 попыток в сутки",
        ],
        paragraphs: [
          "Если прошло 24 часа с момента создания проверки или превышен лимит попыток, статус становится failed_investigation.",
        ],
      },
      {
        title: "История попыток",
        paragraphs: [
          "Каждая попытка логируется в index_check_history: result (success/error/timeout), duration_ms, response_data или error_message.",
          "Эти данные используются для истории в UI и для аналитики.",
        ],
      },
      {
        title: "Фильтры и сортировка",
        listType: "bullet",
        listItems: [
          "Статусы (multi-select).",
          "Период from/to.",
          "Фильтр isIndexed (true/false).",
          "Поиск по domain_id или URL.",
          "Сортировка по дате, домену, статусу и другим колонкам.",
        ],
      },
      {
        title: "Ручной запуск",
        paragraphs: [
          "Ручной запуск доступен на уровне домена, проекта и для администратора в глобальном списке.",
          "После запуска статус сбрасывается в pending и проверка выполняется по расписанию.",
          "Ручной запуск доступен только для опубликованных доменов.",
        ],
      },
      {
        title: "Админ-возможности",
        adminOnly: true,
        listType: "bullet",
        listItems: [
          "Глобальный список всех проверок по всем доменам.",
          "Фильтр по домену и поиск по URL/ID.",
          "Запуск проверки вручную для любого домена.",
          "Отдельный список проблемных проверок (failed_investigation).",
        ],
        paragraphs: ["Раздел доступен только пользователям с ролью администратора."],
      },
      {
        title: "Статистика и календарь",
        paragraphs: [
          "Агрегаты показывают процент индексации, среднее число попыток до успеха, failed_investigation за период и дневную динамику.",
        ],
      },
    ],
  } satisfies DocsPageContent,

  indexingApi: {
    title: "API проверок индексации",
    description: "Примеры запросов для мониторинга индексации.",
    intro: "Примеры API-запросов для мониторинга индексации в дополнение к Swagger UI.",
    sections: [
      {
        paragraphs: [
          "Все запросы требуют cookie access_token. В curl указывайте cookie вручную (значение можно взять из DevTools).",
        ],
      },
      {
        title: "Список проверок по домену",
        codeBlocks: [
          {
            code: `curl -s \\
  -H "Cookie: access_token=YOUR_TOKEN" \\
  "http://localhost:8080/api/domains/{domainId}/index-checks?status=success,checking&is_indexed=true&from=2026-02-01&to=2026-02-12&search=example.com&sort=check_date:desc&limit=20&page=1"`,
            caption: "Ответ: { items: IndexCheck[], total: number }",
          },
        ],
      },
      {
        title: "История попыток",
        codeBlocks: [
          {
            code: `curl -s \\
  -H "Cookie: access_token=YOUR_TOKEN" \\
  "http://localhost:8080/api/domains/{domainId}/index-checks/{checkId}/history?limit=50"`,
          },
        ],
      },
      {
        title: "Статистика и календарь",
        codeBlocks: [
          {
            code: `curl -s \\
  -H "Cookie: access_token=YOUR_TOKEN" \\
  "http://localhost:8080/api/domains/{domainId}/index-checks/stats?from=2026-02-01&to=2026-02-12"`,
          },
          {
            code: `curl -s \\
  -H "Cookie: access_token=YOUR_TOKEN" \\
  "http://localhost:8080/api/domains/{domainId}/index-checks/calendar?month=2026-02"`,
          },
        ],
      },
      {
        title: "Запуск вручную (домен)",
        codeBlocks: [
          {
            code: `curl -s -X POST \\
  -H "Cookie: access_token=YOUR_TOKEN" \\
  "http://localhost:8080/api/domains/{domainId}/index-checks"`,
            caption: "В ответе доступны поля run_now_enqueued и run_now_error.",
          },
        ],
      },
      {
        title: "Проектные проверки",
        codeBlocks: [
          {
            code: `curl -s \\
  -H "Cookie: access_token=YOUR_TOKEN" \\
  "http://localhost:8080/api/projects/{projectId}/index-checks?status=success&limit=20&page=1"`,
          },
          {
            code: `curl -s -X POST \\
  -H "Cookie: access_token=YOUR_TOKEN" \\
  "http://localhost:8080/api/projects/{projectId}/index-checks"`,
            caption:
              "Ответ содержит counters: created, updated, skipped, upsert_failed, enqueued, enqueue_failed.",
          },
        ],
      },
      {
        title: "Admin-эндпоинты",
        codeBlocks: [
          {
            code: `curl -s \\
  -H "Cookie: access_token=YOUR_TOKEN" \\
  "http://localhost:8080/api/admin/index-checks?domain_id={domainId}&limit=20&page=1"`,
          },
          {
            code: `curl -s -X POST \\
  -H "Content-Type: application/json" \\
  -H "Cookie: access_token=YOUR_TOKEN" \\
  -d '{ "domain_id": "{domainId}" }' \\
  "http://localhost:8080/api/admin/index-checks/run"`,
          },
          {
            code: `curl -s \\
  -H "Cookie: access_token=YOUR_TOKEN" \\
  "http://localhost:8080/api/admin/index-checks/failed?limit=20&page=1"`,
          },
        ],
      },
    ],
  } satisfies DocsPageContent,
};

export type DocsPageKey = keyof typeof docsPages;
