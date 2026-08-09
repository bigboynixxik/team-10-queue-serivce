import { useQuery } from '@tanstack/react-query';
import { useNavigate } from 'react-router-dom';

import type { Product } from '@entities/product';
import { queueQueries } from '@entities/queue';
import { appPath } from '@shared/config';

export const useJoinQueueProductCard = (product: Product) => {
  const navigate = useNavigate();
  const { data: membership } = useQuery(queueQueries.me(product.id));
  const isQueued = membership?.status === 'QUEUED';

  const openProduct = () => navigate(appPath(`/order-info/${product.id}`));

  return { isQueued, openProduct };
};
