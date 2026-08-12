import { afterEach, beforeEach, describe, expect, rs, test } from '@rstest/core';
import { act, renderHook } from '@testing-library/react';

import { USER_ID_STORAGE_KEY, USER_ROLE_STORAGE_KEY } from '@shared/config';
import { resetAllStores } from '@shared/model';

rs.mock('./utils', () => ({
  createUserId: () => 'initial-user',
}));

const { useUserStore } = await import('./store');

describe('useUserStore', () => {
  beforeEach(() => {
    localStorage.clear();
    resetAllStores();
  });

  afterEach(() => {
    localStorage.clear();
    resetAllStores();
  });

  test('starts with createUserId value and buyer role', () => {
    expect(useUserStore.getState().userId).toBe('initial-user');
    expect(useUserStore.getState().role).toBe('buyer');
  });

  test('setUserId updates state and localStorage', () => {
    act(() => {
      useUserStore.getState().setUserId('next-user');
    });

    expect(useUserStore.getState().userId).toBe('next-user');
    expect(localStorage.getItem(USER_ID_STORAGE_KEY)).toBe('next-user');
  });

  test('setRole updates state and localStorage', () => {
    act(() => {
      useUserStore.getState().setRole('seller');
    });

    expect(useUserStore.getState().role).toBe('seller');
    expect(localStorage.getItem(USER_ROLE_STORAGE_KEY)).toBe('seller');
  });

  test('reset restores initial user id and role', () => {
    act(() => {
      useUserStore.getState().setUserId('next-user');
      useUserStore.getState().setRole('seller');
      useUserStore.getState().reset();
    });

    expect(useUserStore.getState().userId).toBe('initial-user');
    expect(useUserStore.getState().role).toBe('buyer');
  });

  test('selector returns current userId', () => {
    const { result } = renderHook(() => useUserStore.use.userId());

    expect(result.current).toBe('initial-user');

    act(() => {
      useUserStore.getState().setUserId('selected-user');
    });

    expect(result.current).toBe('selected-user');
  });
});
