import { queryOptions } from '@tanstack/react-query';

import { APP_STALE_TIME } from '@shared/config';

import { productApi } from '../api/ProductApi';

export const productListQueryKey = () => ['products', 'list'] as const;

export const productQueryKey = (id: string) => ['products', 'detail', id] as const;

export const productQueries = {
  list: () =>
    queryOptions({
      queryKey: productListQueryKey(),
      queryFn: () => productApi.list(),
      staleTime: APP_STALE_TIME,
    }),
  byId: (id: string) =>
    queryOptions({
      queryKey: productQueryKey(id),
      queryFn: () => productApi.byId(id),
      staleTime: APP_STALE_TIME,
    }),
};
