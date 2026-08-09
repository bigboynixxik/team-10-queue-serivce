# Frontend — Авито Очередь

Клиент сервиса пользовательской очереди для покупки дефицитных товаров (кейс хакатона Авито).

Стек: **React + TypeScript**, сборка на **Rsbuild**, архитектура **Feature-Sliced Design (FSD)**, тесты на **Rstest**.

---

## Использование нейросетей в проекте

При разработке использовался `Opus 5` с целью проверить возможные обходы pipline-а очереди и поиска уязвимостей в проекте. Так же использовался `Cursor Grok 4.5` для создания документации к проекту и небольших фиксов. Так же использовался `GPT-5.6 Terra` для создания не критических unit-тестов. 

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
- `@rstest/core` ^0.11.6
- `@rstest/adapter-rsbuild` ^0.11.6
- `@rstest/coverage-istanbul` ^0.11.6
- `@testing-library/react` ^16.3.2
- `@testing-library/jest-dom` ^7.0.0
- `happy-dom` ^20.11.2

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
| `npm run test` | Юнит-тесты (Rstest, один прогон) |
| `npm run test:watch` | Тесты в watch-режиме |
| `npm run test:coverage` | Тесты + coverage (Istanbul) |

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
| `widgets` | Крупные блоки UI (хедер, каталог, CTA очереди) |
| `features` | Пользовательские сценарии (встать/выйти из очереди, оплата) |
| `entities` | Бизнес-сущности и их API/модель (`queue`, `product`, `user`) |
| `shared` | UI-kit, HTTP/WS-клиенты, конфиг, утилиты |

Внутри слайса типичные сегменты: `ui`, `model`, `api`. Публичный API слайса — через `index.ts`.

Алиасы путей (см. `rsbuild.config.ts` / `tsconfig.json`): `@app`, `@pages`, `@widgets`, `@features`, `@entities`, `@shared`, `@ui`, `@test`.

---

## Структура папок

```
frontend/
├── biome.json              # Biome: lint + format
├── steiger.config.js       # Steiger: правила FSD
├── rsbuild.config.ts       # Сборка и алиасы
├── rstest.config.ts        # Rstest: env, setup, coverage
├── tsconfig.json
├── .env.example
├── docker/                 # Dockerfile для prod
├── nginx/                  # Конфиг раздачи статики + proxy /api
├── test/                   # Общие хелперы тестов (не FSD)
│   ├── setup.ts            # jest-dom matchers + RTL cleanup
│   ├── render.tsx          # render / renderHook + QueryClient
│   └── mockAxios.ts        # фабрика мока axios
└── src/
    ├── app/                # bootstrap, providers, router, layout
    ├── pages/              # home, order-info, payment-success
    ├── widgets/            # header, product-catalog, order-queue-cta
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

## Тесты

### Стек и конфиг

| Часть | Выбор |
| --- | --- |
| Раннер | [`@rstest/core`](https://rstest.dev/) + `@rstest/adapter-rsbuild` (те же алиасы, что у сборки) |
| DOM | `happy-dom` |
| React | `@testing-library/react` + `@testing-library/jest-dom` |
| Coverage | Istanbul (`@rstest/coverage-istanbul`), репортёры `text` и `html` |

Конфиг: `rstest.config.ts`.

- `include`: `src/**/*.{test,spec}.{ts,tsx}`
- `setupFiles`: `./test/setup.ts`
- coverage считает `src/**/*.{ts,tsx}`, исключая сами тесты и `*.module.css`

```bash
npm run test              # один прогон
npm run test:watch        # watch
npm run test:coverage     # + отчёт coverage (html в coverage/)
```

### Спека: что покрываем

Фокус — **model / api / lib**, а не разметка. UI-компоненты тестируем только если в них есть нетривиальная логика; страницы и виджеты в основном композиция и остаются без snapshot-тестов.

| Слой | Что тестируем | Примеры |
| --- | --- | --- |
| `shared` | HTTP/WS-клиенты, маппинг ошибок, утилиты URL | `HttpClient`, `WebSocketClient`, `HttpError`, `getWsUrl` |
| `entities` | Zod-схемы API, клиенты REST/WS, query/mutation keys и хуки, store, live-updates | `MembershipSchema`, `QueueApi`, `RightsApi`, `useMembershipLiveUpdates`, user store |
| `features` | сценарии хуков: join/leave, payment/checkout, статус очереди, my-queues, offer-actions | `useJoinQueue`, `usePayment`, `useQueueStatus`, `useOfferActions` |
| `pages` | model страницы (параметры роута → состояние) | `useOrderInfoPage` |
| `widgets` | чистые хелперы рядом с UI | `formatTime` |

### Спека: соглашения

1. **Colocation** — файл `foo.ts` → соседний `foo.test.ts` (не общая папка `__tests__`).
2. **Импорты из `@rstest/core`** — `describe`, `test`, `expect`, `beforeEach`, `rs` (не Vitest/Jest).
3. **Моки до импорта модуля** — `rs.mock(...)`, затем динамический `await import('./moduleUnderTest')`, чтобы моки применились к зависимостям.
4. **Частичный мок пакета** — `import * as pkg with { rstest: 'importActual' }` и спред в фабрике `rs.mock`.
5. **HTTP** — `createAxiosMock()` из `@test/mockAxios`, не реальные запросы.
6. **React Query** — `renderHookWithProviders` / `renderWithProviders` из `@test/render` (retry выключен, свой `QueryClient`).
7. **WebSocket / globals** — `rs.stubGlobal` + fake в тесте, в `afterEach` — `rs.unstubAllGlobals()`.
8. **Изоляция** — в `beforeEach` сбрасывать `rs.fn()` и локальное состояние; `localStorage` чистить при работе с user id.
9. **Ассерты** — поведение и контракты (вызов mutate с payload, парсинг схемы, статус membership), без привязки к CSS-классам и текстам кнопок, если это не часть контракта UX-ошибки.

### Хелперы (`test/`)

| Файл | Назначение |
| --- | --- |
| `setup.ts` | `expect.extend(jest-dom)` + `cleanup()` после каждого теста |
| `render.tsx` | `createTestQueryClient`, `renderWithProviders`, `renderHookWithProviders` |
| `mockAxios.ts` | `createAxiosMock()` — `get/post/put/patch/delete` + interceptors |

Алиас: `@test/*` → `./test/*`.

### Шаблон нового теста

```ts
import { beforeEach, describe, expect, rs, test } from '@rstest/core';
import { renderHook } from '@testing-library/react';

const mutate = rs.fn();

rs.mock('@some/dep', () => ({
  useSomething: () => ({ mutate }),
}));

const { useUnderTest } = await import('./useUnderTest');

describe('useUnderTest', () => {
  beforeEach(() => {
    mutate.mockReset();
  });

  test('does the thing', () => {
    const { result } = renderHook(() => useUnderTest());
    // arrange / act / assert
    expect(mutate).toHaveBeenCalled();
  });
});
```

Для хуков с QueryClient — `@test/render` вместо «голого» `renderHook`. Для API-клиентов — `@test/mockAxios` и мок `axios` / `@shared/config`.

### Что пока вне скоупа

- E2E / Playwright
- Визуальные / snapshot-тесты UI-kit
- Принудительный порог coverage в CI (coverage доступен через `npm run test:coverage`)

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
