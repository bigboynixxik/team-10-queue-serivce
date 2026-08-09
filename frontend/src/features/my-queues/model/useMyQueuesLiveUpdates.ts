import { useQuery } from '@tanstack/react-query';
import { useCallback, useRef } from 'react';

import { productQueries } from '@entities/product';
import { type UserQueue, useUserQueuesLiveUpdates } from '@entities/queue';
import { useToast } from '@ui';

import { describeUserQueuesUpdate } from './describeUserQueuesUpdate';

const LIVE_UPDATE_TOAST_DURATION = 4000;

export const useMyQueuesLiveUpdates = (userId: string): void => {
  const { info } = useToast();
  const { data: products } = useQuery(productQueries.list());
  const previousQueues = useRef<UserQueue[] | undefined>(undefined);

  const onUpdate = useCallback(
    (queues: UserQueue[]) => {
      const previous = previousQueues.current;
      previousQueues.current = queues;

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
