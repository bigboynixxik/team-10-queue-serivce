import { afterEach, beforeEach, describe, expect, rs, test } from '@rstest/core';

import { USER_ID_STORAGE_KEY } from '@shared/config';

import { createUserId } from './utils';

const UUID_V4_RE = /^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$/;

describe('createUserId', () => {
  beforeEach(() => {
    localStorage.clear();
  });

  afterEach(() => {
    rs.restoreAllMocks();
    localStorage.clear();
  });

  test('returns stored user id', () => {
    localStorage.setItem(USER_ID_STORAGE_KEY, 'stored-user');

    expect(createUserId()).toBe('stored-user');
  });

  test('generates and stores a new user id when missing', () => {
    const generatedId = '123e4567-e89b-12d3-a456-426614174000' as const;
    rs.spyOn(crypto, 'randomUUID').mockReturnValue(generatedId);

    expect(createUserId()).toBe(generatedId);
    expect(localStorage.getItem(USER_ID_STORAGE_KEY)).toBe(generatedId);
  });

  test('falls back to getRandomValues when randomUUID is unavailable', () => {
    Object.defineProperty(crypto, 'randomUUID', {
      configurable: true,
      value: undefined,
    });

    const userId = createUserId();

    expect(userId).toMatch(UUID_V4_RE);
    expect(localStorage.getItem(USER_ID_STORAGE_KEY)).toBe(userId);
  });
});
