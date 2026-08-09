import { useMutation } from '@tanstack/react-query';

import { queueMutations } from '@entities/queue';
import { useErrorNotifier } from '@shared/lib';

export const useLeaveQueue = (productId: string) => {
  const notifyError = useErrorNotifier();
  // выход из очереди это тот же delete members/me
  const mutation = useMutation(queueMutations.declineOffer(productId));

  const leaveQueue = () => {
    mutation.mutate(undefined, {
      onError: (error) => notifyError(error, 'Не удалось выйти из очереди'),
    });
  };

  return { leaveQueue, isPending: mutation.isPending };
};
