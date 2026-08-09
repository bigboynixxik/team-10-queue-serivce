import { beforeEach, describe, expect, rs, test } from '@rstest/core';
import * as reactQuery from '@tanstack/react-query' with { rstest: 'importActual' };
import { renderHook } from '@testing-library/react';

let params: { productId?: string } = {};
let queryState = {
  data: undefined as { id: string } | undefined,
  isPending: false,
  isError: false,
};

rs.mock('react-router-dom', () => ({
  useParams: () => params,
}));

rs.mock('@tanstack/react-query', () => ({
  ...reactQuery,
  useQuery: () => queryState,
}));

const { useOrderInfoPage } = await import('./useOrderInfoPage');

describe('useOrderInfoPage', () => {
  beforeEach(() => {
    params = {};
    queryState = { data: undefined, isPending: false, isError: false };
  });

  test('marks missing productId as error and not pending', () => {
    const { result } = renderHook(() => useOrderInfoPage());

    expect(result.current.isPending).toBe(false);
    expect(result.current.isError).toBe(true);
    expect(result.current.product).toBeUndefined();
  });

  test('returns product and pending state when id is present', () => {
    params = { productId: 'p1' };
    queryState = { data: { id: 'p1' }, isPending: true, isError: false };

    const { result } = renderHook(() => useOrderInfoPage());

    expect(result.current.product).toEqual({ id: 'p1' });
    expect(result.current.isPending).toBe(true);
    expect(result.current.isError).toBe(false);
  });

  test('propagates query error', () => {
    params = { productId: 'p1' };
    queryState = { data: undefined, isPending: false, isError: true };

    const { result } = renderHook(() => useOrderInfoPage());

    expect(result.current.isError).toBe(true);
  });
});
