import { queryOptions } from '@tanstack/react-query';

import { queueApi } from '../api/QueueApi';

export const queueMembershipQueryKey = (productId: string) =>
  ['queue', 'membership', productId] as const;

export const queueQueries = {
  me: (productId: string) =>
    queryOptions({
      queryKey: queueMembershipQueryKey(productId),
      queryFn: () => queueApi.getMe(productId),
      retry: false,
    }),
};
