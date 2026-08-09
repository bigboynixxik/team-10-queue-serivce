import { useQuery } from '@tanstack/react-query';
import { useNavigate } from 'react-router-dom';

import type { Product } from '@entities/product';
import { queueQueries } from '@entities/queue';
import { useUserStore } from '@entities/user';
import { appPath } from '@shared/config';

export const useJoinQueueProductCard = (product: Product) => {
  const navigate = useNavigate();
  const userId = useUserStore.use.userId();
  // один /me/queues на каталог вместо N× /members/me
  const { data: queues = [] } = useQuery(queueQueries.allForUser(userId));
  const isQueued = queues.some(
    (queue) => queue.product_id === product.id && queue.status === 'QUEUED',
  );

  const openProduct = () => navigate(appPath(`/order-info/${product.id}`));

  return { isQueued, openProduct };
};
