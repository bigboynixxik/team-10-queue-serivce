import { describe, expect, test } from '@rstest/core';

import { membershipRefetchInterval } from './membershipRefetchInterval';

describe('membershipRefetchInterval', () => {
  test('returns false for terminal statuses', () => {
    expect(membershipRefetchInterval('DECLINED')).toBe(false);
    expect(membershipRefetchInterval('PURCHASED')).toBe(false);
    expect(membershipRefetchInterval('SOLD_OUT')).toBe(false);
  });

  test('returns 2000 for active or missing status', () => {
    expect(membershipRefetchInterval('QUEUED')).toBe(2000);
    expect(membershipRefetchInterval('RIGHT_ACTIVE')).toBe(2000);
    expect(membershipRefetchInterval(undefined)).toBe(2000);
  });
});
