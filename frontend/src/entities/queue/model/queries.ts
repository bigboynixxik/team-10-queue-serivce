import { queryOptions } from '@tanstack/react-query';

import { queueApi } from '../api/QueueApi';
import { userQueuesApi } from '../api/UserQueuesApi';

export const queueMembershipQueryKey = (productId: string) =>
  ['queue', 'membership', productId] as const;

export const userQueuesQueryKey = (userId: string) => ['queue', 'user-queues', userId] as const;

export const queueStatsQueryKey = (productId: string) => ['queue', 'stats', productId] as const;

export const queueQueries = {
  me: (productId: string) =>
    queryOptions({
      queryKey: queueMembershipQueryKey(productId),
      queryFn: () => queueApi.getMe(productId),
      retry: false,
    }),
  stats: (productId: string) =>
    queryOptions({
      queryKey: queueStatsQueryKey(productId),
      queryFn: () => queueApi.getStats(productId),
      enabled: Boolean(productId),
    }),
  allForUser: (userId: string) =>
    queryOptions({
      queryKey: userQueuesQueryKey(userId),
      queryFn: () => userQueuesApi.getAll(),
      enabled: Boolean(userId),
    }),
};
