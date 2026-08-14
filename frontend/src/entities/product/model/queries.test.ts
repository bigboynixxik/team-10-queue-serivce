import { beforeEach, describe, expect, rs, test } from '@rstest/core';

const list = rs.fn();
const byId = rs.fn();

rs.mock('@shared/config', () => ({
  APP_STALE_TIME: 12345,
}));

rs.mock('../api/ProductApi', () => ({
  productApi: { list, byId },
}));

const { productListQueryKey, productQueries, productQueryKey } = await import('./queries');

describe('productQueries', () => {
  beforeEach(() => {
    list.mockReset();
    byId.mockReset();
  });

  test('builds keys and uses staleTime', async () => {
    list.mockResolvedValue([]);
    byId.mockResolvedValue({ id: 'p1' });

    expect(productListQueryKey()).toEqual(['products', 'list']);
    expect(productQueryKey('p1')).toEqual(['products', 'detail', 'p1']);

    const listOptions = productQueries.list();
    const byIdOptions = productQueries.byId('p1');

    expect(listOptions.staleTime).toBe(12345);
    expect(byIdOptions.staleTime).toBe(12345);

    await listOptions.queryFn?.({} as never);
    await byIdOptions.queryFn?.({} as never);

    expect(list).toHaveBeenCalled();
    expect(byId).toHaveBeenCalledWith('p1');
  });
});
