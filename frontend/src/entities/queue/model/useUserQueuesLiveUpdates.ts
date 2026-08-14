import { type QueryClient, useQueryClient } from '@tanstack/react-query';
import { useEffect, useRef } from 'react';

import { WebSocketClient } from '@shared/api';
import { API_BASE_URL } from '@shared/config';

import { type Membership, type UserQueue, UserQueuesSchema } from '../api/type';
import { queueMembershipQueryKey, userQueuesQueryKey } from './queries';

const reconnectDelays = [1000, 2000, 5000, 10000];

/** Synchronizes product membership caches from the application-wide queue stream. */
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

const getUserQueuesWsUrl = (userId: string): string => {
  const apiUrl = new URL(API_BASE_URL || '/api/v1', window.location.origin);

  apiUrl.protocol = apiUrl.protocol === 'https:' ? 'wss:' : 'ws:';
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
    let socket: WebSocketClient<unknown> | undefined;

    const reconnect = () => {
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

      const nextSocket = new WebSocketClient<unknown>({ url: getUserQueuesWsUrl(userId) });
      socket = nextSocket;

      nextSocket.connect({
        onMessage: (payload) => {
          const result = UserQueuesSchema.safeParse(payload);
          if (!result.success) {
            queryClient.invalidateQueries({ queryKey });
            return;
          }

          reconnectAttempt = 0;
          queryClient.setQueryData(queryKey, result.data);
          syncMemberships(queryClient, result.data);
          onUpdateRef.current?.(result.data);
        },
        onError: () => queryClient.invalidateQueries({ queryKey }),
        onClose: () => {
          if (disposed || socket !== nextSocket) return;

          socket = undefined;
          queryClient.invalidateQueries({ queryKey });
          reconnect();
        },
      });
    };

    connect();

    return () => {
      disposed = true;
      if (reconnectTimer !== undefined) window.clearTimeout(reconnectTimer);
      socket?.disconnect();
    };
  }, [queryClient, userId]);
};
