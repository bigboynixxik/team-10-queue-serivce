# Диаграммы последовательности: Авито Очередь

Четыре сценария покрывают все финальные состояния основного пути (мгновенная покупка / успех после ожидания / отказ из-за распроданности / многоштучный запрос с частичным предложением). Подробное описание каждого пронумерованного перехода — в `docs/user_story.md` (для сценариев 1–3; сценарий 4 пока не задокументирован там отдельно).

В сценариях 1–3 каждый покупатель запрашивает `quantity: 1`, поэтому это поле в запросах и в `product_count` опущено для краткости и показано как `product_count--`. Сценарий 4 — единственный, где `quantity` больше 1 и явно участвует в логике.

Условное обозначение участников везде одинаковое: `A`, `B`, `C` — покупатели (браузер), `QS` — Queue Service (наш backend), `AB` — AvitoBackend (внешний бэкенд Авито, вне скоупа кейса; инкапсулирует все условно существующие API текущего бэкенда Авито — оформление заказа/оплату, а также данные о товаре и его остатке).

Перед созданием заказа в AvitoBackend покупатель напрямую вызывает `GET /rights/{token}` у Queue Service, чтобы проверить своё право — это отсекает невалидные попытки (чужой/просроченный токен) до того, как они дойдут до эндпоинта создания заказа AvitoBackend. Этот шаг показан во всех сценариях перед `create order`.

---

## Сценарий 1 — покупатель один, очереди нет

```mermaid
sequenceDiagram
    autonumber
    participant A as Покупатель A (браузер)
    participant QS as Queue Service (наш backend)
    participant AB as AvitoBackend (внешний, вне скоупа)

    A->>QS: POST /queue/{product_id}/members
    QS-->>A: 201 { status: RIGHT_ACTIVE, token, expires_at }
    Note over A: available_units > 0 и очередь пуста → право выдано мгновенно,<br/>экран ожидания не показывается

    A->>QS: GET /queue/{product_id}/members/me (Upgrade: websocket)
    Note over A: Открывает realtime-канал сразу — он понадобится позже,<br/>чтобы получить асинхронное подтверждение оплаты

    A->>QS: GET /rights/{token}
    QS-->>A: 200 { valid: true }
    Note over A: Проверка права выполняется на стороне QS до обращения к AvitoBackend —<br/>невалидные попытки не долетают до эндпоинта создания заказа

    A->>AB: create order (token передан, вне скоупа)
    AB-->>A: форма оплаты
    A->>AB: оплата
    AB-->>QS: POST /rights/{token}/events { event: payment_succeeded, order_id }

    QS->>QS: right(A) = USED
    QS->>AB: PATCH /products/{product_id}/stock { decrement: 1 }
    QS-->>AB: 202 Accepted
    QS-->>A: WS push { status: PURCHASED }
```

---

## Сценарий 2 — гонка, второй покупатель получает право и покупает

Товар с одним свободным слотом (`total_stock = 1`) в момент конфликта. A первым получает право, но бездействует до истечения таймера; право переходит к B по FIFO, и B успевает оплатить.

```mermaid
sequenceDiagram
    autonumber
    participant A as Покупатель A (браузер)
    participant B as Покупатель B (браузер)
    participant QS as Queue Service (наш backend)
    participant AB as AvitoBackend (внешний, вне скоупа)

    A->>QS: POST /queue/{product_id}/members
    QS-->>A: 201 { status: RIGHT_ACTIVE, token, expires_at }
    Note over A: available_units был > 0 → право выдано мгновенно,<br/>экран ожидания пропущен, сразу переход к оформлению

    A->>QS: GET /queue/{product_id}/members/me (Upgrade: websocket)
    Note over A: Открывает realtime-канал сразу — понадобится,<br/>чтобы узнать об истечении своего права или об успешной оплате

    B->>QS: POST /queue/{product_id}/members
    QS-->>B: 201 { status: QUEUED }
    B->>QS: GET /queue/{product_id}/members/me (Upgrade: websocket)
    Note over B: available_units стал 0 → B встаёт в очередь,<br/>видит экран ожидания. Текст сообщения формирует фронтенд по status, не backend

    opt Демонстрация: попытка обхода очереди
        B->>QS: GET /rights/{token_A} (пробует обойти очередь чужим токеном)
        QS-->>B: 403 Forbidden
        Note over QS: Токен принадлежит A, а не B — Queue Service отклоняет проверку<br/>напрямую, до какого-либо обращения к AvitoBackend
    end

    alt A успевает оформить и оплатить заказ до истечения таймера
        Note over A: См. Сценарий 1 — здесь этот путь не происходит
    else A бездействует до истечения таймера (этот сценарий)
        Note over A: A бездействует и не завершает оформление

        QS->>QS: right(A).expires_at достигнут → right(A) = EXPIRED
        QS-->>A: WS push { status: QUEUED }
        Note over QS: Слот освободился → выдаём право следующему в FIFO (B)

        QS-->>B: WS push { status: RIGHT_ACTIVE, token, expires_at }
        Note over B: Экран меняется на «Ваша очередь», таймер запущен,<br/>кнопка оформления разблокирована
    end

    B->>QS: GET /rights/{token}
    QS-->>B: 200 { valid: true }

    B->>AB: create order (token передан, вне скоупа)
    AB-->>B: форма оплаты
    B->>AB: оплата
    AB-->>QS: POST /rights/{token}/events { event: payment_succeeded, order_id }

    QS->>QS: right(B) = USED
    QS->>AB: PATCH /products/{product_id}/stock { decrement: 1 }
    QS-->>AB: 202 Accepted
    QS-->>B: WS push { status: PURCHASED }
    Note over QS: Если это была последняя единица и очередь пуста —<br/>товар помечается SOLD_OUT для новых покупателей
```

---

## Сценарий 3 — гонка, опоздавшие получают SOLD_OUT

Тот же товар с `total_stock = 1`. A получает право первым и сразу оплачивает — быстрее, чем истекает таймер. B и C встают в очередь следом за A и никогда не получат право, потому что единица уже продана.

```mermaid
sequenceDiagram
    autonumber
    participant A as Покупатель A (браузер)
    participant B as Покупатель B (браузер)
    participant C as Покупатель C (браузер)
    participant QS as Queue Service (наш backend)
    participant AB as AvitoBackend (внешний, вне скоупа)

    A->>QS: POST /queue/{product_id}/members
    QS-->>A: 201 { status: RIGHT_ACTIVE, token, expires_at }
    Note over A: available_units был 1 → становится 0, право выдано мгновенно

    A->>QS: GET /queue/{product_id}/members/me (Upgrade: websocket)
    Note over A: Открывает realtime-канал сразу — понадобится для<br/>асинхронного подтверждения оплаты

    B->>QS: POST /queue/{product_id}/members
    QS-->>B: 201 { status: QUEUED }
    B->>QS: GET /queue/{product_id}/members/me (Upgrade: websocket)

    C->>QS: POST /queue/{product_id}/members
    QS-->>C: 201 { status: QUEUED }
    C->>QS: GET /queue/{product_id}/members/me (Upgrade: websocket)
    Note over QS: FIFO-порядок: B — первый в очереди, C — второй

    alt A успевает оплатить до истечения таймера (этот сценарий)
        A->>QS: GET /rights/{token}
        QS-->>A: 200 { valid: true }

        A->>AB: create order (token передан, вне скоупа)
        AB-->>A: форма оплаты
        A->>AB: оплата (успевает до истечения таймера)
        AB-->>QS: POST /rights/{token}/events { event: payment_succeeded, order_id }

        QS->>QS: right(A) = USED, product_count = 0
        QS->>AB: PATCH /products/{product_id}/stock { decrement: 1 }
        QS-->>AB: 202 Accepted
        QS-->>A: WS push { status: PURCHASED }
        Note over QS: product_count == 0 и активных прав больше нет →<br/>товар SOLD_OUT для всех, кто ещё в очереди
    else A бездействует / не успевает
        Note over A: См. Сценарий 2 — здесь этот путь не происходит
    end

    par
        QS-->>B: WS push { status: SOLD_OUT }
    and
        QS-->>C: WS push { status: SOLD_OUT }
    end
```

---

## Сценарий 4 — несколько единиц товара, частичное предложение и отказ

Товар с `total_stock = 4`. A берёт 1 штуку сразу и полностью завершает покупку (create order → оплата → `payment_succeeded`) до того, как B входит в очередь — именно поэтому к моменту входа B доступно уже 3, а не 4. B хочет 5, но доступно только 3 — получает предложение (`OFFER_PENDING`) сразу при входе (очередь для него пуста) и соглашается на 2. C хочет 2, встаёт в очередь, т.к. на момент его входа всё оставшееся удержано за B; когда B оплачивает, остаётся только 1 единица — C получает предложение уже на неё и отказывается, оставляя эту единицу непроданной.

```mermaid
sequenceDiagram
    autonumber
    participant A as Покупатель A (браузер)
    participant B as Покупатель B (браузер)
    participant C as Покупатель C (браузер)
    participant QS as Queue Service (наш backend)
    participant AB as AvitoBackend (внешний, вне скоупа)

    QS->>AB: GET /products/{product_id}/stock
    AB-->>QS: { available: 4 }
    Note over QS: Единоразовая инициализация локального состояния QS для товара —<br/>дальше QS ведёт available_units/product_count как кэш этого значения

    A->>QS: POST /queue/{product_id}/members { quantity: 1 }
    QS-->>A: 201 { status: RIGHT_ACTIVE, token, expires_at, quantity: 1 }
    Note over A: available_units = 3 сразу после выдачи права A (ещё не оплачен)

    A->>QS: GET /queue/{product_id}/members/me (Upgrade: websocket)

    A->>QS: GET /rights/{token}
    QS-->>A: 200 { valid: true }

    A->>AB: create order (token передан, вне скоупа)
    AB-->>A: форма оплаты
    A->>AB: оплата
    AB-->>QS: POST /rights/{token}/events { event: payment_succeeded, order_id }

    QS->>QS: right(A) = USED
    QS->>AB: PATCH /products/{product_id}/stock { decrement: 1 }
    QS-->>AB: 202 Accepted
    QS-->>A: WS push { status: PURCHASED }
    Note over QS: A полностью завершил покупку до входа B — available_units остаётся 3.<br/>Это и объясняет, почему B ниже получит OFFER_PENDING вместо RIGHT_ACTIVE на 5 единиц

    B->>QS: POST /queue/{product_id}/members { quantity: 5 }
    QS-->>B: 201 { status: OFFER_PENDING, available_quantity: 3, expires_at }
    Note over B: Очередь для B пуста, но доступно (3) меньше запрошенного (5) →<br/>предложение выдаётся сразу, а не после ожидания

    B->>QS: GET /queue/{product_id}/members/me (Upgrade: websocket)

    C->>QS: POST /queue/{product_id}/members { quantity: 2 }
    QS-->>C: 201 { status: QUEUED }
    Note over QS: Все 3 доступные единицы временно удержаны за B до его решения — C ждёт

    C->>QS: GET /queue/{product_id}/members/me (Upgrade: websocket)

    alt B соглашается на меньшее количество (этот сценарий)
        Note over B: B решает купить меньше, чем предложено — 2 из 3

        B->>QS: PATCH /queue/{product_id}/members/me { quantity: 2 }
        QS-->>B: 200 { status: RIGHT_ACTIVE, token, expires_at, quantity: 2 }
        Note over QS: 1 неиспользованная единица из удержанных B возвращается в пул,<br/>но этого пока недостаточно для полного запроса C (2) — очередь не продвигается

        B->>QS: GET /rights/{token}
        QS-->>B: 200 { valid: true }

        B->>AB: create order (token передан, вне скоупа)
        AB-->>B: форма оплаты
        B->>AB: оплата
        AB-->>QS: POST /rights/{token}/events { event: payment_succeeded, order_id }

        QS->>QS: right(B) = USED
        QS->>AB: PATCH /products/{product_id}/stock { decrement: 2 }
        QS-->>AB: 202 Accepted
        QS-->>B: WS push { status: PURCHASED }
        Note over QS: Остался 1 экземпляр, впереди в очереди — C (просил 2)
    else B отказывается полностью
        Note over B: См. отказ C ниже — механика та же (DELETE → WS push DECLINED),<br/>токен не выдан, заказ не создаётся
    end

    QS-->>C: WS push { status: OFFER_PENDING, available_quantity: 1, expires_at }
    Note over C: Доступно (1) меньше запрошенного (2) — снова предложение, а не полное право

    alt C отказывается от предложения (этот сценарий)
        Note over C: C решает, что 1 книга ей не подходит, отказывается

        C->>QS: DELETE /queue/{product_id}/members/me
        QS-->>C: 204 No Content
        QS-->>C: WS push { status: DECLINED }
        Note over QS: Очередь пуста, 1 единица остаётся нераспроданной — сценарий завершён
    else C соглашается на меньшее количество
        Note over C: См. принятие B выше — механика та же (PATCH → RIGHT_ACTIVE)
    end
```
