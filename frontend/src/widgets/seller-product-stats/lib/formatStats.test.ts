import { describe, expect, test } from '@rstest/core';

import { formatDeficit, formatDuration, formatMoney } from './formatStats';

describe('formatStats', () => {
  test('formats money in rubles', () => {
    expect(formatMoney(12990)).toContain('₽');
    expect(formatMoney(12990)).toContain('12');
  });

  test('formats duration', () => {
    expect(formatDuration(45)).toBe('45 с');
    expect(formatDuration(120)).toBe('2 мин');
    expect(formatDuration(95)).toBe('1 мин 35 с');
  });

  test('formats deficit coefficient', () => {
    expect(formatDeficit(10, 200, 20)).toBe('на 10 шт. претендуют 200 чел. (×20)');
  });
});
