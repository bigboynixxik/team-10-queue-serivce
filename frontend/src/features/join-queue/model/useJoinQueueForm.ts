import type { Product } from '@entities/product';

import { useJoinQueue } from './useJoinQueue';

export const useJoinQueueForm = (product: Product) => {
  const { join, isPending } = useJoinQueue(product);

  return {
    submit: () => join(1),
    isPending,
  };
};
