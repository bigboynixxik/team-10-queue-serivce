import { useMutation } from '@tanstack/react-query';

import type { Product } from '@entities/product';
import { queueMutations } from '@entities/queue';
import { useErrorNotifier } from '@shared/lib';

export const useJoinQueue = (product: Product) => {
  const notifyError = useErrorNotifier();
  const mutation = useMutation(queueMutations.join(product.id));

  const join = (quantity: number) => {
    mutation.mutate(
      { quantity },
      {
        onError: (error) => notifyError(error, 'Не удалось встать в очередь'),
      },
    );
  };

  return { join, isPending: mutation.isPending };
};
