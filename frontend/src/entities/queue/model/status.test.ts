import { describe, expect, test } from '@rstest/core';

import { isTerminalStatus } from './status';

describe('isTerminalStatus', () => {
  test.each(['DECLINED', 'PURCHASED', 'SOLD_OUT'] as const)(
    'returns true for terminal status %s',
    (status) => {
      expect(isTerminalStatus(status)).toBe(true);
    },
  );

  test.each(['QUEUED', 'RIGHT_ACTIVE', 'OFFER_PENDING'] as const)(
    'returns false for active status %s',
    (status) => {
      expect(isTerminalStatus(status)).toBe(false);
    },
  );

  test('returns false for undefined', () => {
    expect(isTerminalStatus(undefined)).toBe(false);
  });
});
