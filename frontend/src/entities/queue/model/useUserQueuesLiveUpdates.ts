import { API_BASE_URL } from '@shared/config';
import { useQueryClient } from '@tanstack/react-query';
import { useEffect, useRef } from 'react';

import { type UserQueue, UserQueuesSchema } from '../api/type';
import { userQueuesQueryKey } from './queries';

const reconnectDelays = [1000, 2000, 5000, 10000];

const getUserQueuesSseUrl = (userId: string): string => {
  const apiUrl = new URL(API_BASE_URL || '/api/v1', window.location.origin);

  apiUrl.pathname = `${apiUrl.pathname.replace(/\/$/, '')}/me/queues`;
  apiUrl.searchParams.set('user_id', userId);

  return apiUrl.toString();
};

type Options = {
  /** Called with the fresh snapshot after every `update` event. */
  onUpdate?: (queues: UserQueue[], previous?: UserQueue[]) => void;
};

export const useUserQueuesLiveUpdates = (userId: string, { onUpdate }: Options = {}): void => {
  const queryClient = useQueryClient();
  const onUpdateRef = useRef(onUpdate);

  // Kept in a ref so a new callback identity never restarts the stream.
  onUpdateRef.current = onUpdate;

  useEffect(() => {
    if (!userId) return;

    const queryKey = userQueuesQueryKey(userId);
    let disposed = false;
    let reconnectAttempt = 0;
    let reconnectTimer: number | undefined;
    let source: EventSource | undefined;

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

      const nextSource = new EventSource(getUserQueuesSseUrl(userId));
      source = nextSource;

      nextSource.addEventListener('update', (event) => {
        try {
          const queues = UserQueuesSchema.parse(JSON.parse((event as MessageEvent<string>).data));
          const previous = queryClient.getQueryData<UserQueue[]>(queryKey);

          reconnectAttempt = 0;
          queryClient.setQueryData(queryKey, queues);
          onUpdateRef.current?.(queues, previous);
        } catch {
          queryClient.invalidateQueries({ queryKey });
        }
      });

      nextSource.onerror = () => {
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
