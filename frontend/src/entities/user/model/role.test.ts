import { describe, expect, test } from '@rstest/core';

import { USER_ROLE_STORAGE_KEY } from '@shared/config';

import { isUserRole, readStoredRole } from './role';

describe('role utils', () => {
  test('isUserRole accepts only buyer and seller', () => {
    expect(isUserRole('buyer')).toBe(true);
    expect(isUserRole('seller')).toBe(true);
    expect(isUserRole('admin')).toBe(false);
    expect(isUserRole(null)).toBe(false);
  });

  test('readStoredRole falls back to buyer', () => {
    localStorage.clear();
    expect(readStoredRole()).toBe('buyer');

    localStorage.setItem(USER_ROLE_STORAGE_KEY, 'seller');
    expect(readStoredRole()).toBe('seller');
  });
});
