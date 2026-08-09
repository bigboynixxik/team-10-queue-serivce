# Диаграмма контейнеров C4 (уровень 2)

Диаграмма описывает исполняемые единицы решения, их зоны ответственности и протоколы взаимодействия. Состав контейнеров соответствует файлу `docker-compose.yml`; внешняя система AvitoBackend в развёртывании замещена заглушкой `backend/cmd/avitomock`.

Обработчик истечения сроков не выделен в отдельный контейнер: он исполняется отдельной горутиной внутри процесса прикладного интерфейса и запускается в `backend/cmd/api/main.go`.

```mermaid
flowchart TD
classDef person fill:#08427b,stroke:#052e56,color:#ffffff,font-weight:bold,rx:50,ry:50
classDef container fill:#1168bd,stroke:#0b4884,color:#ffffff,font-weight:bold,rx:10,ry:10
classDef database fill:#1168bd,stroke:#0b4884,color:#ffffff,font-weight:bold,rx:10,ry:10
classDef external fill:#999999,stroke:#6b6b6b,color:#ffffff,font-weight:bold,rx:10,ry:10

User("Покупатель<br/>[Пользователь]<br/>Выбирает товар, ожидает в очереди, принимает предложения и оплачивает заказ"):::person

AvitoBackend("AvitoBackend<br/>[Внешняя система, в развёртывании — заглушка avitomock, Go]<br/>Владеет физическим остатком, оформляет заказ и проводит оплату"):::external

subgraph System ["Система «Авито Очередь» (Queue Service)"]
direction TB

WebApp("Веб-приложение<br/>[Контейнер: React 19, TypeScript, nginx]<br/>Отображает состояния членства, обслуживает каталог<br/>и проксирует /api к прикладному интерфейсу"):::container

QueueAPI("Прикладной интерфейс очереди<br/>[Контейнер: Go 1.26, net/http]<br/>Обрабатывает REST-запросы, обслуживает каналы WebSocket и SSE,<br/>исполняет конечный автомат очереди"):::container

Worker("Обработчик истечения сроков<br/>[Горутина внутри контейнера прикладного интерфейса]<br/>Освобождает просроченные права и предложения,<br/>инициирует продвижение очереди"):::container

Redis("Кэш и горячий путь<br/>[Контейнер: Redis 7]<br/>Очередь FIFO, атомарное распределение остатка,<br/>кэш членства и прав, таймеры, публикация событий"):::database

Postgres("Долговременное хранилище<br/>[Контейнер: PostgreSQL 16]<br/>Таблицы rights, queue_memberships, product_stock"):::database
end

User -- "Работает с интерфейсом очереди<br/>[HTTPS / WSS]" --> WebApp
User -- "Подтверждает оплату на странице оформления заказа<br/>[HTTPS]" --> AvitoBackend

WebApp -- "Вход в очередь (POST), принятие части (PATCH),<br/>выход (DELETE), показатели спроса (GET)<br/>[JSON / HTTP]" --> QueueAPI
WebApp -. "Состояние членства в конкретной очереди<br/>[WebSocket]" .-> QueueAPI
WebApp -. "Перечень очередей пользователя<br/>[Server-Sent Events]" .-> QueueAPI
WebApp -- "Переход на страницу оформления заказа<br/>с непрозрачным идентификатором права (UUID)<br/>[HTTPS]" --> AvitoBackend

AvitoBackend -- "Сообщает о факте успешной оплаты<br/>POST /rights/{token}/events, заголовок X-Internal-Token<br/>[JSON / HTTP]" --> QueueAPI
QueueAPI -- "Читает остаток и сообщает о его уменьшении<br/>GET и PATCH /products/{id}/stock, заголовок X-Internal-Token<br/>[JSON / HTTP]" --> AvitoBackend

QueueAPI -- "Атомарное распределение остатка сценариями Lua,<br/>очередь FIFO, кэш состояний, публикация событий<br/>[RESP / TCP]" --> Redis
QueueAPI -- "Фиксирует выданные права и состояния членства,<br/>оплата и истечение срока — транзакцией<br/>[SQL / TCP]" --> Postgres

Worker -- "Читает и снимает просроченные таймеры,<br/>возвращает удержанные единицы в пул<br/>[RESP / TCP]" --> Redis
Worker -- "Переводит право в EXPIRED и фиксирует<br/>терминальное состояние членства одной транзакцией<br/>[SQL / TCP]" --> Postgres
```

## Разграничение ответственности хранилищ

Redis и PostgreSQL решают различные задачи, взаимная замена которых невозможна.

Предотвращение состязания за последнюю единицу товара обеспечивается сценарием Lua в Redis: решение о выдаче принимается атомарно в рамках единственной операции. PostgreSQL в принятии решения не участвует.

Сохранность принятого решения обеспечивается PostgreSQL. Порядок операций записи фиксирован: сначала долговременное хранилище, затем кэш. Подробное изложение приведено в `docs/storage/redis.md` и `docs/storage/postgres.md`.

## Разграничение доступа

| Группа конечных точек | Инициатор вызова | Механизм контроля |
| --- | --- | --- |
| `/api/v1/queue/*`, `/api/v1/me/queues`, `GET /api/v1/rights/{token}` | Браузер покупателя | Заголовок `X-User-Id`; для каналов реального времени — параметр запроса `user_id` |
| `POST /api/v1/rights/{token}/events` | Внешняя система AvitoBackend | Заголовок `X-Internal-Token` |
| `GET /api/v1/queue/{product_id}/stats`, `/healthz` | Произвольный | Отсутствует |

Право на покупку представлено непрозрачным значением UUID и не содержит кодированных данных: принадлежность устанавливается поиском записи и сравнением её поля `user_id` с идентификатором обратившегося (`docs/design_context.md`, п. 4).
