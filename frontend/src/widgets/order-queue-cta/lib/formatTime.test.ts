import { describe, expect, test } from '@rstest/core';

import { formatEta, formatTimeLeft } from './formatTime';

describe('formatTimeLeft', () => {
  test('pads minutes and seconds', () => {
    expect(formatTimeLeft(0)).toBe('00:00');
    expect(formatTimeLeft(65)).toBe('01:05');
    expect(formatTimeLeft(600)).toBe('10:00');
  });
});

describe('formatEta', () => {
  test('returns less than a minute for short eta', () => {
    expect(formatEta(0)).toBe('менее минуты');
    expect(formatEta(59)).toBe('менее минуты');
  });

  test('formats whole minutes', () => {
    expect(formatEta(120)).toBe('2 мин.');
  });

  test('formats minutes with remaining seconds', () => {
    expect(formatEta(125)).toBe('2 мин. 5 сек.');
  });
});
