package main

import "html/template"

// checkoutTemplate is the stand-in for the Avito checkout page. It only needs to
// prove one thing: the purchase is completed outside Queue Service, which learns
// about it through an event rather than by being part of the payment path
// (docs/design_context.md, п. 7).
var checkoutTemplate = template.Must(template.New("checkout").Parse(`<!doctype html>
<html lang="ru">
<head>
<meta charset="utf-8">
<title>Оформление заказа — Авито</title>
<style>
  body { font-family: system-ui, -apple-system, sans-serif; max-width: 30rem;
         margin: 4rem auto; padding: 0 1rem; color: #1a1a1a; }
  .card { border: 1px solid #e0e0e0; border-radius: 12px; padding: 1.5rem; }
  .muted { color: #707070; font-size: .875rem; }
  code { background: #f3f3f3; padding: .15rem .35rem; border-radius: 4px;
         word-break: break-all; font-size: .8125rem; }
  button { background: #04e061; border: 0; border-radius: 8px; color: #1a1a1a;
           font-size: 1rem; font-weight: 600; padding: .75rem 1.5rem;
           cursor: pointer; width: 100%; margin-top: 1.5rem; }
  button:disabled { background: #e0e0e0; cursor: default; }
  #result { margin-top: 1rem; font-size: .9375rem; }
  .ok { color: #0a7d33; }
  .err { color: #c4102e; }
</style>
</head>
<body>
  <div class="card">
    <h1>Оформление заказа</h1>
    <p class="muted">Заглушка внешнего чекаута Авито. Реальной оплаты здесь нет —
       кнопка сообщает сервису очереди, что оплата прошла.</p>
    <p class="muted">Товар: <code>{{.ProductID}}</code></p>
    <p class="muted">Право: <code>{{.Token}}</code></p>
    <button id="pay">Оплатить</button>
    <div id="result"></div>
  </div>
<script>
  const token = {{.Token}};
  const returnUrl = {{.ReturnURL}};
  const button = document.getElementById('pay');
  const result = document.getElementById('result');

  button.addEventListener('click', async () => {
    button.disabled = true;
    result.textContent = 'Отправляем...';
    try {
      const response = await fetch('/checkout/pay', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ token }),
      });
      const body = await response.json();
      if (response.ok) {
        result.className = 'ok';
        result.textContent = 'Оплачено. Возвращаем к товарам...';
        window.setTimeout(() => window.location.assign(returnUrl), 800);
      } else {
        result.className = 'err';
        result.textContent = 'Не вышло: ' + (body.error || response.status);
        button.disabled = false;
      }
    } catch (e) {
      result.className = 'err';
      result.textContent = 'Ошибка сети: ' + e.message;
      button.disabled = false;
    }
  });
</script>
</body>
</html>`))
