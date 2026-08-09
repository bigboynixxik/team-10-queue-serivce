import { type QueryClient, useQueryClient } from '@tanstack/react-query';
import { useEffect, useRef } from 'react';

import { API_BASE_URL } from '@shared/config';

import { type Membership, type UserQueue, UserQueuesSchema } from '../api/type';
import { queueMembershipQueryKey, userQueuesQueryKey } from './queries';

const reconnectDelays = [1000, 2000, 5000, 10000];

/** пишет membership-кэши из sse списка очередей */
const syncMemberships = (queryClient: QueryClient, queues: UserQueue[]): void => {
  for (const queue of queues) {
    queryClient.setQueryData<Membership>(queueMembershipQueryKey(queue.product_id), {
      status: queue.status,
      token: queue.token,
      quantity: queue.quantity,
      available_quantity: queue.available_quantity,
      expires_at: queue.expires_at,
    });
  }
};

const getUserQueuesSseUrl = (userId: string): string => {
  const apiUrl = new URL(API_BASE_URL || '/api/v1', window.location.origin);

  apiUrl.pathname = `${apiUrl.pathname.replace(/\/$/, '')}/me/queues`;
  apiUrl.searchParams.set('user_id', userId);

  return apiUrl.toString();
};

type Options = {
  onUpdate?: (queues: UserQueue[]) => void;
};

export const useUserQueuesLiveUpdates = (userId: string, { onUpdate }: Options = {}): void => {
  const queryClient = useQueryClient();
  const onUpdateRef = useRef(onUpdate);

  onUpdateRef.current = onUpdate;

  useEffect(() => {
    if (!userId) return;

    const queryKey = userQueuesQueryKey(userId);
    let disposed = false;
    let reconnectAttempt = 0;
    let reconnectTimer: number | undefined;
    let source: EventSource | undefined;

    const reconnect = () => {
      // второй reconnect не ставим пока тикает таймер
      if (disposed || reconnectTimer !== undefined) return;

      const delay = reconnectDelays[Math.min(reconnectAttempt, reconnectDelays.length - 1)];
      reconnectAttempt += 1;
      reconnectTimer = window.setTimeout(() => {
        reconnectTimer = undefined;
        connect();
      }, delay);
    };

    const connect = () => {
      if (disposed) return;

      const nextSource = new EventSource(getUserQueuesSseUrl(userId));
      source = nextSource;

      nextSource.addEventListener('update', (event) => {
        try {
          const queues = UserQueuesSchema.parse(JSON.parse((event as MessageEvent<string>).data));

          reconnectAttempt = 0;
          queryClient.setQueryData(queryKey, queues);
          syncMemberships(queryClient, queues);
          onUpdateRef.current?.(queues);
        } catch {
          queryClient.invalidateQueries({ queryKey });
        }
      });

      nextSource.onerror = () => {
        // onerror от уже заменённого source игнорим
        if (disposed || source !== nextSource) return;

        nextSource.close();
        source = undefined;
        queryClient.invalidateQueries({ queryKey });
        reconnect();
      };
    };

    connect();

    return () => {
      disposed = true;
      if (reconnectTimer !== undefined) window.clearTimeout(reconnectTimer);
      source?.close();
    };
  }, [queryClient, userId]);
};
