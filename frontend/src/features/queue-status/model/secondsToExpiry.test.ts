import { afterEach, describe, expect, rs, test } from '@rstest/core';

import { secondsToExpiry } from './secondsToExpiry';

describe('secondsToExpiry', () => {
  afterEach(() => {
    rs.useRealTimers();
  });

  test('returns null when expiresAt is missing', () => {
    expect(secondsToExpiry()).toBeNull();
    expect(secondsToExpiry(undefined)).toBeNull();
  });

  test('returns 0 when expiry is in the past', () => {
    rs.useFakeTimers();
    rs.setSystemTime(new Date('2026-08-09T12:00:00.000Z'));

    expect(secondsToExpiry('2026-08-09T11:59:00.000Z')).toBe(0);
  });

  test('returns ceil seconds until expiry', () => {
    rs.useFakeTimers();
    rs.setSystemTime(new Date('2026-08-09T12:00:00.000Z'));

    expect(secondsToExpiry('2026-08-09T12:00:01.100Z')).toBe(2);
  });
});
