import { queryOptions } from '@tanstack/react-query';

import { APP_STALE_TIME } from '@shared/config';

import { sellerStatsApi } from '../api/SellerStatsApi';

export const sellerStatsQueryKey = (productId: string) =>
  ['seller-stat', 'detail', productId] as const;

export const sellerStatsQueries = {
  byProductId: (productId: string) =>
    queryOptions({
      queryKey: sellerStatsQueryKey(productId),
      queryFn: () => sellerStatsApi.byProductId(productId),
      staleTime: APP_STALE_TIME,
    }),
};
