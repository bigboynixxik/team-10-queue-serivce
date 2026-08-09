import { describe, expect, test } from '@rstest/core';

import { describeShortage } from './describeShortage';

describe('describeShortage', () => {
  test('describes remaining stock without requested quantity', () => {
    expect(describeShortage(3)).toBe('Осталось только 3 шт.');
  });

  test('describes shortage against requested quantity', () => {
    expect(describeShortage(2, 5)).toBe('Вы выбрали 5 шт., а осталось только 2 шт.');
  });
});
