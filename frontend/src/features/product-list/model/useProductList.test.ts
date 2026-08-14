import { describe, expect, rs, test } from '@rstest/core';
import * as reactQuery from '@tanstack/react-query' with { rstest: 'importActual' };
import { renderHook } from '@testing-library/react';

const useQuery = rs.fn(() => ({ data: [{ id: 'p1' }], isPending: false }));

rs.mock('@entities/product', () => ({
  productQueries: {
    list: () => ({ queryKey: ['products', 'list'], staleTime: 30000 }),
  },
}));

rs.mock('@tanstack/react-query', () => ({
  ...reactQuery,
  useQuery,
}));

const { useProductList } = await import('./useProductList');

describe('useProductList', () => {
  test('queries product list options', () => {
    const { result } = renderHook(() => useProductList());

    expect(useQuery).toHaveBeenCalledWith({
      queryKey: ['products', 'list'],
      staleTime: 30000,
    });
    expect(result.current.data).toEqual([{ id: 'p1' }]);
  });
});
