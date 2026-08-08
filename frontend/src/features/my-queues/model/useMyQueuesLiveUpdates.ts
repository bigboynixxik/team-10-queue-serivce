import { productQueries } from '@entities/product';
import { type UserQueue, useUserQueuesLiveUpdates } from '@entities/queue';
import { useQuery } from '@tanstack/react-query';
import { useToast } from '@ui';
import { useCallback } from 'react';

import { describeUserQueuesUpdate } from './describeUserQueuesUpdate';

// Live updates arrive as often as the queues move, so they stay on screen far
// shorter than the status toasts a user has to act upon.
const LIVE_UPDATE_TOAST_DURATION = 4000;

/**
 * Mounted once for the whole app: keeps the single SSE stream open and reports
 * every event as a toast, whether or not the queues menu is on screen.
 */
export const useMyQueuesLiveUpdates = (userId: string): void => {
  const { info } = useToast();
  const { data: products } = useQuery(productQueries.list());

  const onUpdate = useCallback(
    (queues: UserQueue[], previous?: UserQueue[]) => {
      const getProductTitle = (productId: string) =>
        products?.find((product) => product.id === productId)?.title ?? productId;

      info(
        {
          title: 'Обновление очередей',
          description: describeUserQueuesUpdate(queues, previous, getProductTitle),
        },
        { duration: LIVE_UPDATE_TOAST_DURATION },
      );
    },
    [info, products],
  );

  useUserQueuesLiveUpdates(userId, { onUpdate });
};
