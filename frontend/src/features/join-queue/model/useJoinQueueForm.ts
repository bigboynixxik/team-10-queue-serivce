import { useQuery } from '@tanstack/react-query';
import { useState } from 'react';

import type { Product } from '@entities/product';
import { queueQueries } from '@entities/queue';
import type { Nullable } from '@shared/model';
import { useToast } from '@ui';

import { useJoinQueue } from './useJoinQueue';

const MIN_QUANTITY = 1;

export const useJoinQueueForm = (product: Product) => {
  const { join, isPending } = useJoinQueue(product);
  const toast = useToast();
  const { data: stats } = useQuery(queueQueries.stats(product.id));
  const [quantity, setQuantity] = useState(MIN_QUANTITY);

  const maxQuantity = stats?.product_count;

  const notifyOverLimit = (max: number) =>
    toast.error(
      max > 0
        ? `Столько товаров купить нельзя. Максимум — ${max} шт.`
        : 'Товара больше нет в наличии.',
    );

  const exceededLimit = (value: number): Nullable<number> =>
    maxQuantity !== undefined && value > maxQuantity ? maxQuantity : null;

  const decrease = () => setQuantity((current) => Math.max(MIN_QUANTITY, current - 1));

  const increase = () => {
    const limit = exceededLimit(quantity + 1);

    if (limit !== null) {
      notifyOverLimit(limit);
      return;
    }

    setQuantity(quantity + 1);
  };

  const submit = () => {
    const limit = exceededLimit(quantity);

    if (limit !== null) {
      notifyOverLimit(limit);
      return;
    }

    join(quantity);
  };

  return {
    quantity,
    minQuantity: MIN_QUANTITY,
    increase,
    decrease,
    submit,
    isPending,
  };
};
