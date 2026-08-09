import { useQuery } from '@tanstack/react-query';
import { useEffect, useState } from 'react';

import { type Membership, queueQueries, useMembershipLiveUpdates } from '@entities/queue';
import { useUserStore } from '@entities/user';
import type { Nullable } from '@shared/model';

import { membershipRefetchInterval } from './membershipRefetchInterval';
import { secondsToExpiry } from './secondsToExpiry';

type QueueStatus = {
  membership: Nullable<Membership>;
  secondsLeft: Nullable<number>;
  isPending: boolean;
  isError: boolean;
};

export const useQueueStatus = (productId: string): QueueStatus => {
  const userId = useUserStore.use.userId();
  const { data, isPending, isError } = useQuery({
    ...queueQueries.me(productId),
    enabled: Boolean(productId),
    refetchInterval: (query) => membershipRefetchInterval(query.state.data?.status),
  });
  const membership = data ?? null;
  const expiresAt = membership?.expires_at;
  const [secondsLeft, setSecondsLeft] = useState<Nullable<number>>(null);

  useMembershipLiveUpdates(productId, userId, membership?.status);

  useEffect(() => {
    setSecondsLeft(secondsToExpiry(expiresAt));

    if (!expiresAt) return;

    const timer = window.setInterval(() => setSecondsLeft(secondsToExpiry(expiresAt)), 1000);

    return () => window.clearInterval(timer);
  }, [expiresAt]);

  return { membership, secondsLeft, isPending, isError };
};
