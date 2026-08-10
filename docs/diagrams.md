# Диаграммы последовательности: «Авито Очередь»

Четыре сценария покрывают все терминальные состояния основного пути: немедленная покупка, покупка после ожидания, отказ вследствие исчерпания остатка, многоштучный запрос с частичным предложением. Обоснование принятых решений приведено в `docs/design_context.md`, форма прикладного интерфейса — в `docs/api.yml`.

Обозначения участников едины для всех сценариев: `A`, `B`, `C` — покупатели (браузер), `QS` — Queue Service, `AB` — AvitoBackend, внешняя система Авито, находящаяся вне области кейса и инкапсулирующая оформление заказа, оплату, а также данные о товаре и его остатке.

В сценариях 1–3 каждый покупатель запрашивает одну единицу, вследствие чего поле `quantity` в запросах опущено для краткости. Сценарий 4 — единственный, в котором запрашиваемое количество превышает единицу и участвует в логике распределения.

Защита от обхода очереди изображена двумя шагами. Покупатель обращается к `GET /rights/{token}` перед переходом к оформлению заказа: проверка отсекает недействительные попытки — предъявление чужого либо истёкшего права — до того, как они достигнут внешней системы. Затем сама AvitoBackend обращается к `POST /internal/rights/{token}/validate` и создаёт заказ исключительно при получении кода `204`. Первый шаг является клиентским и может быть пропущен, второй представляет собой границу доверия и пропущен быть не может (`docs/design_context.md`, п. 5.8).

Вход в очередь при непустой очереди приводит к состоянию `QUEUED` независимо от наличия свободных единиц: освободившиеся единицы принадлежат голове очереди, а не вновь пришедшему. Условие проверяется тем же атомарным шагом, что и распределение остатка.

Уведомление AvitoBackend об уменьшении остатка (`PATCH /products/{product_id}/stock`) отправляется после ответа `202`, а не до него: событие фиксируется той же транзакцией, что и оплата, а доставляет его фоновый обработчик почтового ящика, повторяя попытки до успеха.

Обращение `GET /products/{product_id}/stock` к AvitoBackend выполняется при обработке входа в очередь, не завершившегося идемпотентным возвратом существующего членства; полученное значение применяется исключительно при отсутствии локального состояния товара (`docs/design_context.md`, п. 9). Для краткости шаг показан только в сценарии 4, где значение остатка существенно для понимания последующих переходов.

Мгновенные состояния сопровождаются уведомлением по каналу реального времени. Публикуемое в Redis сообщение является сигналом инвалидации: обработчик повторно читает текущее состояние и передаёт клиенту фактическое представление. На диаграммах показан результат, то есть состояние, полученное клиентом.

Блоки `alt`, `opt` и `par` применяются в точках действительного ветвления — принятие решения пользователем, состязание «успел либо не успел», — а не для перечисления всех теоретически возможных исходов одного вызова.

---

## Сценарий 1 — единственный покупатель, очередь не образуется

```mermaid
sequenceDiagram
    autonumber
    participant A as Покупатель A (браузер)
    participant QS as Queue Service
    participant AB as AvitoBackend (внешний, вне скоупа)

    A->>QS: POST /queue/{product_id}/members
    QS-->>A: 201 { status: RIGHT_ACTIVE, token, expires_at, quantity }
    Note over A: available_units >= quantity → право выдано немедленно,<br/>экран ожидания не отображается

    A->>QS: GET /queue/{product_id}/members/me (Upgrade: websocket)
    Note over A: Канал открывается сразу — он необходим позже,<br/>чтобы получить асинхронное подтверждение оплаты

    A->>QS: GET /rights/{token}
    QS-->>A: 200 { valid: true }
    Note over A: Проверка выполняется на стороне QS до обращения к AvitoBackend —<br/>недействительные попытки не достигают конечной точки оформления заказа

    A->>AB: Создание заказа (право предъявлено, вне скоупа)
    AB->>QS: POST /internal/rights/{token}/validate { product_id }
    QS-->>AB: 204 No Content
    Note over AB: Граница доверия: заказ создаётся только при 204.<br/>Пропустить эту проверку браузер не может

    AB-->>A: Форма оплаты
    A->>AB: Оплата
    AB-->>QS: POST /rights/{token}/events { event: payment_succeeded, order_id }

    QS->>QS: Транзакция PostgreSQL: right(A) = USED, product_count -= quantity,<br/>membership(A) = PURCHASED, событие в stock_decrement_outbox
    QS-->>AB: 202 Accepted
    QS-->>A: WS: { status: PURCHASED }

    QS->>AB: PATCH /products/{product_id}/stock { decrement: 1 }, Idempotency-Key
    Note over QS: Уведомление доставляет фоновый обработчик почтового ящика,<br/>повторяя попытки до успеха — сетевой сбой его не теряет
```

---

## Сценарий 2 — состязание, право переходит второму покупателю

Товар с единственной свободной единицей. Покупатель A получает право первым, но бездействует до истечения срока; право переходит покупателю B в порядке FIFO, и B завершает оплату.

```mermaid
sequenceDiagram
    autonumber
    participant A as Покупатель A (браузер)
    participant B as Покупатель B (браузер)
    participant QS as Queue Service
    participant AB as AvitoBackend (внешний, вне скоупа)

    A->>QS: POST /queue/{product_id}/members
    QS-->>A: 201 { status: RIGHT_ACTIVE, token, expires_at, quantity }
    Note over A: available_units был больше нуля → право выдано немедленно,<br/>экран ожидания пропущен

    A->>QS: GET /queue/{product_id}/members/me (Upgrade: websocket)
    Note over A: Канал необходим, чтобы узнать об истечении срока<br/>собственного права либо об успешной оплате

    B->>QS: POST /queue/{product_id}/members
    QS-->>B: 201 { status: QUEUED, quantity }
    B->>QS: GET /queue/{product_id}/members/me (Upgrade: websocket)
    QS-->>B: WS: { status: QUEUED, position: 1, eta_seconds }
    Note over B: available_units стал равен нулю → B помещён в очередь.<br/>Текст сообщения формирует клиентское приложение по состоянию

    opt Демонстрация: попытка обхода очереди
        B->>QS: GET /rights/{token_A}
        QS-->>B: 403 Forbidden
        Note over QS: Право принадлежит A — QS отклоняет проверку<br/>до какого-либо обращения к AvitoBackend
    end

    alt A завершает оформление и оплату до истечения срока
        Note over A: См. сценарий 1 — данный путь здесь не реализуется
    else A бездействует до истечения срока (рассматриваемый сценарий)
        Note over A: A не завершает оформление

        QS->>QS: Достигнут expires_at → транзакция PostgreSQL:<br/>right(A) = EXPIRED, membership(A) = DECLINED
        QS-->>A: WS: { status: DECLINED }
        Note over A: Состояние терминально — соединение закрывается сервером.<br/>Для новой попытки требуется повторный вход в очередь

        Note over QS: Единица возвращена в пул → продвижение очереди FIFO
        QS-->>B: WS: { status: RIGHT_ACTIVE, token, expires_at, quantity }
        Note over B: Экран сменяется, отсчёт запущен,<br/>переход к оформлению разблокирован
    end

    Note over QS,B: Аналогичный переход наступает досрочно, если B закрывает вкладку:<br/>без подтверждения присутствия в течение RIGHT_HEARTBEAT_TIMEOUT<br/>право освобождается, не дожидаясь исчерпания RIGHT_TTL

    B->>QS: GET /rights/{token}
    QS-->>B: 200 { valid: true }

    B->>AB: Создание заказа (право предъявлено, вне скоупа)
    AB->>QS: POST /internal/rights/{token}/validate { product_id }
    QS-->>AB: 204 No Content
    AB-->>B: Форма оплаты
    B->>AB: Оплата
    AB-->>QS: POST /rights/{token}/events { event: payment_succeeded, order_id }

    QS->>QS: Транзакция PostgreSQL: right(B) = USED, product_count -= 1,<br/>membership(B) = PURCHASED, событие в stock_decrement_outbox
    QS-->>AB: 202 Accepted
    QS-->>B: WS: { status: PURCHASED }
    QS->>AB: PATCH /products/{product_id}/stock { decrement: 1 }, Idempotency-Key
    Note over QS: Остаток исчерпан → входящие в очередь получают SOLD_OUT
```

---

## Сценарий 3 — состязание, опоздавшие получают SOLD_OUT

Тот же товар с единственной свободной единицей. Покупатель A получает право первым и завершает оплату до истечения срока. Покупатели B и C помещены в очередь следом и права не получат, поскольку единица продана.

```mermaid
sequenceDiagram
    autonumber
    participant A as Покупатель A (браузер)
    participant B as Покупатель B (браузер)
    participant C as Покупатель C (браузер)
    participant QS as Queue Service
    participant AB as AvitoBackend (внешний, вне скоупа)

    A->>QS: POST /queue/{product_id}/members
    QS-->>A: 201 { status: RIGHT_ACTIVE, token, expires_at, quantity }
    Note over A: available_units был равен 1 → становится 0, право выдано немедленно

    A->>QS: GET /queue/{product_id}/members/me (Upgrade: websocket)

    B->>QS: POST /queue/{product_id}/members
    QS-->>B: 201 { status: QUEUED, quantity }
    B->>QS: GET /queue/{product_id}/members/me (Upgrade: websocket)

    C->>QS: POST /queue/{product_id}/members
    QS-->>C: 201 { status: QUEUED, quantity }
    C->>QS: GET /queue/{product_id}/members/me (Upgrade: websocket)
    Note over QS: Порядок FIFO: B — первый в очереди, C — второй

    alt A завершает оплату до истечения срока (рассматриваемый сценарий)
        A->>QS: GET /rights/{token}
        QS-->>A: 200 { valid: true }

        A->>AB: Создание заказа (право предъявлено, вне скоупа)
        AB->>QS: POST /internal/rights/{token}/validate { product_id }
        QS-->>AB: 204 No Content
        AB-->>A: Форма оплаты
        A->>AB: Оплата
        AB-->>QS: POST /rights/{token}/events { event: payment_succeeded, order_id }

        QS->>QS: Транзакция PostgreSQL: right(A) = USED, product_count = 0,<br/>membership(A) = PURCHASED, событие в stock_decrement_outbox
        QS-->>AB: 202 Accepted
        QS-->>A: WS: { status: PURCHASED }
        QS->>AB: PATCH /products/{product_id}/stock { decrement: 1 }, Idempotency-Key
        Note over QS: Продвижение очереди при нулевом остатке →<br/>ожидающие переводятся в терминальное SOLD_OUT
    else A бездействует либо не успевает
        Note over A: См. сценарий 2 — данный путь здесь не реализуется
    end

    par
        QS-->>B: WS: { status: SOLD_OUT }
    and
        QS-->>C: WS: { status: SOLD_OUT }
    end
```

---

## Сценарий 4 — несколько единиц товара, частичное предложение и отказ

Товар с остатком в четыре единицы. Покупатель A приобретает одну единицу и полностью завершает покупку до входа покупателя B, вследствие чего к моменту входа B доступно три единицы. Покупатель B запрашивает пять, получает частичное предложение на три и принимает две. Покупатель C запрашивает две и помещается в очередь, поскольку на момент его входа всё доступное количество удержано за B; возвращённая при частичном принятии единица немедленно направляется C в виде нового предложения, от которого C отказывается. Одна единица остаётся нераспроданной, что является допустимым исходом.

```mermaid
sequenceDiagram
    autonumber
    participant A as Покупатель A (браузер)
    participant B as Покупатель B (браузер)
    participant C as Покупатель C (браузер)
    participant QS as Queue Service
    participant AB as AvitoBackend (внешний, вне скоупа)

    A->>QS: POST /queue/{product_id}/members { quantity: 1 }
    QS->>AB: GET /products/{product_id}/stock
    AB-->>QS: { available: 4 }
    Note over QS: Локальное состояние товара отсутствовало → значение принимается<br/>как исходное, далее QS ведёт product_count и available_units самостоятельно

    QS-->>A: 201 { status: RIGHT_ACTIVE, token, expires_at, quantity: 1 }
    Note over QS: available_units = 3 сразу после выдачи права A,<br/>product_count = 4 до подтверждения оплаты

    A->>QS: GET /queue/{product_id}/members/me (Upgrade: websocket)

    A->>QS: GET /rights/{token}
    QS-->>A: 200 { valid: true }

    A->>AB: Создание заказа (право предъявлено, вне скоупа)
    AB->>QS: POST /internal/rights/{token}/validate { product_id }
    QS-->>AB: 204 No Content
    AB-->>A: Форма оплаты
    A->>AB: Оплата
    AB-->>QS: POST /rights/{token}/events { event: payment_succeeded, order_id }

    QS->>QS: Транзакция PostgreSQL: right(A) = USED, product_count = 3,<br/>membership(A) = PURCHASED, событие в stock_decrement_outbox
    QS-->>AB: 202 Accepted
    QS-->>A: WS: { status: PURCHASED }
    QS->>AB: PATCH /products/{product_id}/stock { decrement: 1 }, Idempotency-Key
    Note over QS: A завершил покупку до входа B — available_units остаётся равным 3.<br/>Это объясняет, почему B получит предложение, а не полное право на 5 единиц

    B->>QS: POST /queue/{product_id}/members { quantity: 5 }
    QS-->>B: 201 { status: OFFER_PENDING, quantity: 5, available_quantity: 3, expires_at }
    Note over B: Доступно (3) меньше запрошенного (5) →<br/>предложение выдаётся немедленно, ожидание не требуется

    B->>QS: GET /queue/{product_id}/members/me (Upgrade: websocket)

    C->>QS: POST /queue/{product_id}/members { quantity: 2 }
    QS-->>C: 201 { status: QUEUED, quantity: 2 }
    Note over QS: available_units = 0: все три единицы удержаны за B<br/>до принятия им решения → C помещён в очередь

    C->>QS: GET /queue/{product_id}/members/me (Upgrade: websocket)
    QS-->>C: WS: { status: QUEUED, position: 1, eta_seconds }

    alt B принимает меньшее количество (рассматриваемый сценарий)
        Note over B: B принимает 2 единицы из 3 предложенных

        B->>QS: PATCH /queue/{product_id}/members/me { quantity: 2 }
        QS-->>B: 200 { status: RIGHT_ACTIVE, token, expires_at, quantity: 2 }

        Note over QS: Неиспользованная единица немедленно возвращается в пул,<br/>после чего выполняется продвижение очереди
        QS-->>C: WS: { status: OFFER_PENDING, quantity: 2, available_quantity: 1, expires_at }
        Note over C: Доступно (1) меньше запрошенного (2) → предложение, а не полное право.<br/>Отсчёт для C начинается здесь, не дожидаясь оплаты B

        B->>QS: GET /rights/{token}
        QS-->>B: 200 { valid: true }

        B->>AB: Создание заказа (право предъявлено, вне скоупа)
        AB->>QS: POST /internal/rights/{token}/validate { product_id }
        QS-->>AB: 204 No Content
        AB-->>B: Форма оплаты
        B->>AB: Оплата
        AB-->>QS: POST /rights/{token}/events { event: payment_succeeded, order_id }

        QS->>QS: Транзакция PostgreSQL: right(B) = USED, product_count = 1,<br/>membership(B) = PURCHASED, событие в stock_decrement_outbox
        QS-->>AB: 202 Accepted
        QS-->>B: WS: { status: PURCHASED }
        QS->>AB: PATCH /products/{product_id}/stock { decrement: 2 }, Idempotency-Key
    else B отказывается полностью
        Note over B: Механика совпадает с отказом C ниже: DELETE → DECLINED,<br/>право не выдаётся, заказ не создаётся, все три единицы возвращаются в пул
    end

    alt C отказывается от предложения (рассматриваемый сценарий)
        Note over C: C отклоняет предложение на одну единицу

        C->>QS: DELETE /queue/{product_id}/members/me
        QS-->>C: 204 No Content
        QS-->>C: WS: { status: DECLINED }
        Note over QS: Единица возвращена в пул, очередь пуста →<br/>product_count = 1 остаётся нераспроданным, сценарий завершён
    else C принимает меньшее количество
        Note over C: Механика совпадает с принятием B выше: PATCH → RIGHT_ACTIVE
    end
```
