import { beforeEach, describe, expect, rs, test } from '@rstest/core';
import * as reactQuery from '@tanstack/react-query' with { rstest: 'importActual' };
import { renderHook } from '@testing-library/react';

let params: { productId?: string } = {};
let role = 'buyer';
let queryState = {
  data: undefined as { id: string } | undefined,
  isPending: false,
  isError: false,
};

rs.mock('react-router-dom', () => ({
  useParams: () => params,
}));

rs.mock('@entities/user', () => ({
  useUserStore: {
    use: {
      role: () => role,
    },
  },
}));

rs.mock('@tanstack/react-query', () => ({
  ...reactQuery,
  useQuery: () => queryState,
}));

const { useOrderInfoPage } = await import('./useOrderInfoPage');

describe('useOrderInfoPage', () => {
  beforeEach(() => {
    params = {};
    role = 'buyer';
    queryState = { data: undefined, isPending: false, isError: false };
  });

  test('marks missing productId as error and not pending', () => {
    const { result } = renderHook(() => useOrderInfoPage());

    expect(result.current.isPending).toBe(false);
    expect(result.current.isError).toBe(true);
    expect(result.current.product).toBeUndefined();
    expect(result.current.isSeller).toBe(false);
  });

  test('returns product and pending state when id is present', () => {
    params = { productId: 'p1' };
    queryState = { data: { id: 'p1' }, isPending: true, isError: false };

    const { result } = renderHook(() => useOrderInfoPage());

    expect(result.current.product).toEqual({ id: 'p1' });
    expect(result.current.isPending).toBe(true);
    expect(result.current.isError).toBe(false);
  });

  test('marks seller role', () => {
    params = { productId: 'p1' };
    role = 'seller';
    queryState = { data: { id: 'p1' }, isPending: false, isError: false };

    const { result } = renderHook(() => useOrderInfoPage());

    expect(result.current.isSeller).toBe(true);
    expect(result.current.role).toBe('seller');
  });

  test('propagates query error', () => {
    params = { productId: 'p1' };
    queryState = { data: undefined, isPending: false, isError: true };

    const { result } = renderHook(() => useOrderInfoPage());

    expect(result.current.isError).toBe(true);
  });
});
