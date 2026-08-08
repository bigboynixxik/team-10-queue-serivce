import type { Product } from '@entities/product';
import { useState } from 'react';

import { useJoinQueue } from './useJoinQueue';

export const useJoinQueueForm = (product: Product) => {
  const [quantity, setQuantity] = useState(1);
  const { join, isPending } = useJoinQueue(product);

  const setValidQuantity = (value: number | null) => setQuantity(Math.max(1, value ?? 1));

  return {
    quantity,
    setQuantity: setValidQuantity,
    submit: () => join(quantity),
    isPending,
  };
};
