import { useQueryClient } from '@tanstack/react-query';
import { useEffect } from 'react';

import { QueueWs } from '../api/QueueWs';
import { queueMembershipQueryKey } from './queries';

/**
 * Pushes websocket updates straight into the React Query cache so the query
 * stays the single source of truth for the membership. A dead socket falls
 * back to a refetch instead of leaving the UI on stale data.
 */
export const useMembershipLiveUpdates = (productId: string, userId: string): void => {
  const queryClient = useQueryClient();

  useEffect(() => {
    if (!productId || !userId) return;

    const queryKey = queueMembershipQueryKey(productId);
    const socket = new QueueWs(productId, userId);

    socket.connect({
      onMembership: (membership) => queryClient.setQueryData(queryKey, membership),
      onError: () => queryClient.invalidateQueries({ queryKey }),
    });

    return () => socket.disconnect();
  }, [productId, userId, queryClient]);
};
