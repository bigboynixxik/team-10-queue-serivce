import { describe, expect, rs, test } from '@rstest/core';

import { resolveCtaAction } from './resolveCtaAction';

describe('resolveCtaAction', () => {
  const pay = rs.fn();
  const submit = rs.fn();
  const goHome = rs.fn();

  test('returns pay action when payable', () => {
    expect(
      resolveCtaAction({
        isPayable: true,
        isQueued: false,
        isSoldOut: false,
        pay,
        submit,
        goHome,
      }),
    ).toEqual({ label: 'Оплатить товар', run: pay });
  });

  test('returns disabled queued action', () => {
    const action = resolveCtaAction({
      isPayable: false,
      isQueued: true,
      isSoldOut: false,
      pay,
      submit,
      goHome,
    });

    expect(action.label).toBe('Вы в очереди');
    expect(action.disabled).toBe(true);
  });

  test('returns go home action when sold out', () => {
    expect(
      resolveCtaAction({
        isPayable: false,
        isQueued: false,
        isSoldOut: true,
        pay,
        submit,
        goHome,
      }),
    ).toEqual({ label: 'Вернуться к товарам', run: goHome });
  });

  test('returns join action by default', () => {
    expect(
      resolveCtaAction({
        isPayable: false,
        isQueued: false,
        isSoldOut: false,
        pay,
        submit,
        goHome,
      }),
    ).toEqual({ label: 'Перейти в очередь', run: submit });
  });
});
