import { useQueryClient } from '@tanstack/react-query';
import { useEffect } from 'react';

import { QueueWs } from '../api/QueueWs';
import type { Membership, MembershipStatus } from '../api/type';
import { queueMembershipQueryKey } from './queries';
import { isTerminalStatus } from './status';

const reconnectDelays = [1000, 2000, 5000, 10000];

export const useMembershipLiveUpdates = (
  productId: string,
  userId: string,
  status?: MembershipStatus,
): void => {
  const queryClient = useQueryClient();

  useEffect(() => {
    // ws только когда уже есть живой membership — иначе 404-петля invalidate→getMe
    if (!productId || !userId || !status || isTerminalStatus(status)) return;

    const queryKey = queueMembershipQueryKey(productId);
    let disposed = false;
    let reconnectAttempt = 0;
    let reconnectTimer: number | undefined;
    let socket: QueueWs | undefined;

    const shouldConnect = () =>
      !disposed && !isTerminalStatus(queryClient.getQueryData<Membership>(queryKey)?.status);

    const connect = () => {
      if (!shouldConnect()) return;

      const nextSocket = new QueueWs(productId, userId);
      socket = nextSocket;

      nextSocket.connect({
        onMembership: (membership) => {
          reconnectAttempt = 0;
          queryClient.setQueryData(queryKey, membership);
        },
        onError: () => queryClient.invalidateQueries({ queryKey }),
        onClose: () => {
          // onclose от уже заменённого сокета игнорим
          if (disposed || socket !== nextSocket) return;

          socket = undefined;
          queryClient.invalidateQueries({ queryKey });

          const delay = reconnectDelays[Math.min(reconnectAttempt, reconnectDelays.length - 1)];
          reconnectAttempt += 1;
          reconnectTimer = window.setTimeout(() => {
            reconnectTimer = undefined;
            connect();
          }, delay);
        },
      });
    };

    connect();

    return () => {
      disposed = true;
      if (reconnectTimer !== undefined) window.clearTimeout(reconnectTimer);
      socket?.disconnect();
    };
  }, [productId, userId, status, queryClient]);
};
