import { describe, expect, test } from '@rstest/core';

import { conflictMessage } from './conflictMessage';

describe('conflictMessage', () => {
  test('maps SOLD_OUT', () => {
    expect(conflictMessage({ status: 'SOLD_OUT' }, 'fallback')).toBe('Товара больше нет');
  });

  test('maps queue_limit_reached with limit', () => {
    expect(conflictMessage({ error: 'queue_limit_reached', limit: 5 }, 'fallback')).toBe(
      'Нельзя стоять больше чем в 5 очередях одновременно. Выйдите из одной, чтобы встать в новую.',
    );
  });

  test('maps queue_limit_reached without limit', () => {
    expect(conflictMessage({ error: 'queue_limit_reached' }, 'fallback')).toBe(
      'Нельзя стоять в таком количестве очередей одновременно. Выйдите из одной, чтобы встать в новую.',
    );
  });

  test('returns fallback for unknown 409 body', () => {
    expect(conflictMessage({}, 'fallback')).toBe('fallback');
    expect(conflictMessage(null, ['a', 'b'])).toEqual(['a', 'b']);
  });
});
