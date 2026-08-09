import { describe, expect, test } from '@rstest/core';

import { isPaymentSucceeded } from './isPaymentSucceeded';

describe('isPaymentSucceeded', () => {
  test('returns true for valid checkout success payload', () => {
    expect(isPaymentSucceeded({ source: 'avito-checkout', event: 'payment_succeeded' })).toBe(true);
  });

  test.each([
    null,
    undefined,
    'payment_succeeded',
    { source: 'other', event: 'payment_succeeded' },
    { source: 'avito-checkout', event: 'payment_failed' },
    { source: 'avito-checkout' },
  ])('returns false for invalid payload %#', (payload) => {
    expect(isPaymentSucceeded(payload)).toBe(false);
  });
});
