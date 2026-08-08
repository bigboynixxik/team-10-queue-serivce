import { useMutation } from '@tanstack/react-query';
import { useNavigate } from 'react-router-dom';

import type { Product } from '@entities/product';
import { queueMutations } from '@entities/queue';
import { useErrorNotifier } from '@shared/lib';

export const useJoinQueue = (product: Product) => {
  const navigate = useNavigate();
  const notifyError = useErrorNotifier();
  const mutation = useMutation(queueMutations.join(product.id));

  const join = (quantity: number) => {
    mutation.mutate(
      { quantity },
      {
        onSuccess: () => navigate(`/queue/${product.id}`),
        onError: (error) => notifyError(error, 'Не удалось встать в очередь'),
      },
    );
  };

  return { join, isPending: mutation.isPending };
};
