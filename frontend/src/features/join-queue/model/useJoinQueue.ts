import type { Product } from '@entities/product';
import { queueMutations } from '@entities/queue';
import { useErrorNotifier } from '@shared/lib';
import { useMutation } from '@tanstack/react-query';

export const useJoinQueue = (product: Product) => {
  const notifyError = useErrorNotifier();
  const mutation = useMutation(queueMutations.join(product.id));

  // The queue is shown on the product page itself, so joining navigates nowhere:
  // the refreshed membership switches the page into its queued state in place.
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
