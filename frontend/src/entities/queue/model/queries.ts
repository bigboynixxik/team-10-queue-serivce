import { queryOptions } from '@tanstack/react-query';

import { HttpError } from '@shared/api';

import { queueApi } from '../api/QueueApi';
import type { Membership } from '../api/type';
import { userQueuesApi } from '../api/UserQueuesApi';

export const queueMembershipQueryKey = (productId: string) =>
  ['queue', 'membership', productId] as const;

export const userQueuesQueryKey = (userId: string) => ['queue', 'user-queues', userId] as const;

export const queueStatsQueryKey = (productId: string) => ['queue', 'stats', productId] as const;

const isNotFound = (error: unknown): boolean => error instanceof HttpError && error.code === 404;

const getMembershipOrNull = async (productId: string): Promise<Membership | null> => {
  try {
    return await queueApi.getMe(productId);
  } catch (error) {
    // 404 = пользователь не в очереди — это нормальный ответ, не ошибка UI
    if (isNotFound(error)) return null;
    throw error;
  }
};

export const queueQueries = {
  me: (productId: string) =>
    queryOptions({
      queryKey: queueMembershipQueryKey(productId),
      queryFn: () => getMembershipOrNull(productId),
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
