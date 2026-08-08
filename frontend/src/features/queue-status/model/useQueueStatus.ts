import { type Membership, queueQueries, useMembershipLiveUpdates } from '@entities/queue';
import { useUserStore } from '@entities/user';
import type { Nullable } from '@shared/model';
import { useQuery } from '@tanstack/react-query';
import { useEffect, useState } from 'react';

const secondsToExpiry = (expiresAt?: string): Nullable<number> => {
  if (!expiresAt) return null;

  return Math.max(0, Math.ceil((new Date(expiresAt).getTime() - Date.now()) / 1000));
};

type QueueStatus = {
  membership: Nullable<Membership>;
  secondsLeft: Nullable<number>;
  isPending: boolean;
  isError: boolean;
};

const isTerminalStatus = (status?: Membership['status']): boolean =>
  status === 'DECLINED' || status === 'PURCHASED' || status === 'SOLD_OUT';

export const useQueueStatus = (productId: string): QueueStatus => {
  const userId = useUserStore.use.userId();
  const { data, isPending, isError } = useQuery({
    ...queueQueries.me(productId),
    enabled: Boolean(productId),
    refetchInterval: (query) => (isTerminalStatus(query.state.data?.status) ? false : 2000),
  });
  const membership = data ?? null;
  const expiresAt = membership?.expires_at;
  const [secondsLeft, setSecondsLeft] = useState<Nullable<number>>(null);

  useMembershipLiveUpdates(productId, userId);

  useEffect(() => {
    setSecondsLeft(secondsToExpiry(expiresAt));

    if (!expiresAt) return;

    const timer = window.setInterval(() => setSecondsLeft(secondsToExpiry(expiresAt)), 1000);

    return () => window.clearInterval(timer);
  }, [expiresAt]);

  return { membership, secondsLeft, isPending, isError };
};
