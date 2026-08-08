import { queueMutations } from '@entities/queue';
import { useErrorNotifier } from '@shared/lib';
import { useMutation } from '@tanstack/react-query';

export const useLeaveQueue = (productId: string) => {
  const notifyError = useErrorNotifier();
  const mutation = useMutation(queueMutations.declineOffer(productId));

  // Leaving happens on the product page, so the user stays there and can join
  // again — for a different quantity, for instance.
  const leaveQueue = () => {
    mutation.mutate(undefined, {
      onError: (error) => notifyError(error, 'Не удалось выйти из очереди'),
    });
  };

  return { leaveQueue, isPending: mutation.isPending };
};
