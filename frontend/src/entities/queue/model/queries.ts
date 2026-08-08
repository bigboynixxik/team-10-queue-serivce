import { queryOptions } from '@tanstack/react-query';

import { queueApi } from '../api/QueueApi';
import { userQueuesApi } from '../api/UserQueuesApi';

export const queueMembershipQueryKey = (productId: string) =>
  ['queue', 'membership', productId] as const;

export const userQueuesQueryKey = (userId: string) => ['queue', 'user-queues', userId] as const;

export const queueQueries = {
  me: (productId: string) =>
    queryOptions({
      queryKey: queueMembershipQueryKey(productId),
      queryFn: () => queueApi.getMe(productId),
      retry: false,
    }),
  allForUser: (userId: string) =>
    queryOptions({
      queryKey: userQueuesQueryKey(userId),
      queryFn: () => userQueuesApi.getAll(),
      enabled: Boolean(userId),
    }),
};
