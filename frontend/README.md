# Frontend — Авито Очередь

Клиент сервиса пользовательской очереди для покупки дефицитных товаров (кейс хакатона Авито).

Стек: **React + TypeScript**, сборка на **Rsbuild**, архитектура **Feature-Sliced Design (FSD)**.

---

## Зависимости

### Runtime

- `react` ^19.2.8
- `react-dom` ^19.2.8
- `react-router-dom` ^7.18.2
- `@tanstack/react-query` ^5.101.4
- `axios` ^1.19.0
- `zustand` ^5.0.14
- `zod` ^3.25.76

### Dev

- `@rsbuild/core` ^2.1.9
- `@rsbuild/plugin-react` ^2.1.0
- `typescript` ^5.9.3
- `@types/react` ^19.2.18
- `@types/react-dom` ^19.2.4
- `@biomejs/biome` ^2.5.6
- `steiger` ^0.6.0
- `@feature-sliced/steiger-plugin` ^0.7.0

---

## Как запустить

### Локально (dev)

Нужны Node.js и npm. API в dev удобнее гонять через nginx из корневого `docker-compose` (он добавляет CORS) — см. комментарии в `.env.example`.

```bash
cd frontend
cp .env.example .env   # при необходимости поправьте URL
npm install
npm run dev
```

Приложение: [http://localhost:3000](http://localhost:3000) (basename маршрутов — `/avito`).

### Скрипты

| Команда | Описание |
| --- | --- |
| `npm run dev` | Dev-сервер Rsbuild |
| `npm run build` | Production-сборка |
| `npm run preview` | Превью собранного бандла |
| `npm run typecheck` | Проверка типов (`tsc --noEmit`) |
| `npm run lint` | Biome (lint + format check) |
| `npm run lint:fix` | Biome с автофиксом |
| `npm run lint:fsd` | Steiger — проверка слоёв FSD |

### Docker

Из корня репозитория:

```bash
docker compose up frontend
```

---

## Переменные окружения

Публичные переменные Rsbuild (`PUBLIC_*`), пример — `.env.example`:

| Переменная | Назначение |
| --- | --- |
| `PUBLIC_API_BASE_URL` | Базовый URL API |
| `PUBLIC_CHECKOUT_BASE_URL` | URL checkout / mock оплаты |
| `PUBLIC_USER_ID_STORAGE_KEY` | Ключ guest user id в `localStorage` |
| `PUBLIC_APP_STALE_TIME` | `staleTime` для React Query (мс) |

---

## Архитектура

Используется **[Feature-Sliced Design](https://feature-sliced.design/)**: код разбит на слои с односторонними зависимостями (сверху вниз).

```
app → pages → widgets → features → entities → shared
```

| Слой | Роль |
| --- | --- |
| `app` | Точка входа, провайдеры, роутер, layout |
| `pages` | Страницы и композиция виджетов/фич |
| `widgets` | Крупные блоки UI (хедер, каталог, сессия очереди) |
| `features` | Пользовательские сценарии (встать/выйти из очереди, оплата) |
| `entities` | Бизнес-сущности и их API/модель (`queue`, `product`, `user`) |
| `shared` | UI-kit, HTTP/WS-клиенты, конфиг, утилиты |

Внутри слайса типичные сегменты: `ui`, `model`, `api`. Публичный API слайса — через `index.ts`.

Алиасы путей (см. `rsbuild.config.ts` / `tsconfig.json`): `@app`, `@pages`, `@widgets`, `@features`, `@entities`, `@shared`, `@ui`.

---

## Структура папок

```
frontend/
├── biome.json              # Biome: lint + format
├── steiger.config.js       # Steiger: правила FSD
├── rsbuild.config.ts       # Сборка и алиасы
├── tsconfig.json
├── .env.example
├── docker/                 # Dockerfile для prod
├── nginx/                  # Конфиг раздачи статики
└── src/
    ├── app/                # bootstrap, providers, router, layout
    ├── pages/              # home, order-info, queue, payment-success
    ├── widgets/            # header, product-catalog, queue-session, …
    ├── features/           # join-queue, leave-queue, payment, my-queues, …
    ├── entities/           # queue, product, user
    └── shared/
        ├── api/            # HttpClient, WebSocketClient, ошибки
        ├── ui/             # переиспользуемые компоненты
        ├── lib/            # query-client, хелперы
        ├── model/          # общие типы / base store helpers
        ├── config/         # env и константы
        └── styles/         # глобальные стили
```

---

## Линтер и качество кода

В проекте два независимых инструмента: **Biome** (код) и **Steiger** (архитектура FSD).

### Biome (`npm run lint`)

Конфиг: `biome.json`.

- Пресет правил: `recommended`
- Домен React: `recommended` (хуки, JSX, типичные антипаттерны React)
- Форматтер: 2 пробела, `lineWidth: 100`, одинарные кавычки, точки с запятой
- `organizeImports` включён через assist

**Зачем:** один инструмент вместо ESLint + Prettier — быстрее CI/локальные проверки, единый стиль и базовый набор безопасных правил без ручной сборки плагинов.

### Steiger + FSD plugin (`npm run lint:fsd`)

Конфиг: `steiger.config.js`.

- База: `@feature-sliced/steiger-plugin` → `recommended`
- `fsd/insignificant-slice` выключен — мелкие слайсы на MVP допустимы
- Для `app/providers` отключён `fsd/segments-by-purpose` — провайдеры живут вне классических сегментов слайса

**Зачем:** Biome не проверяет границы слоёв FSD. Steiger ловит запрещённые импорты (например, `features` → `pages`) и помогает не расползаться архитектуре по мере роста фич.

### TypeScript

`npm run typecheck` — строгий `tsc --noEmit` (`strict`, `verbatimModuleSyntax`). Типы — отдельный слой контроля, не замена линтеру.
