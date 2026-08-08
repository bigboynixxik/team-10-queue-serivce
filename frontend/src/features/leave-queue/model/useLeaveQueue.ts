import { queueMutations } from '@entities/queue';
import { useErrorNotifier } from '@shared/lib';
import { useMutation } from '@tanstack/react-query';
import { useNavigate } from 'react-router-dom';

export const useLeaveQueue = (productId: string) => {
  const notifyError = useErrorNotifier();
  const navigate = useNavigate();
  const mutation = useMutation(queueMutations.declineOffer(productId));

  const leaveQueue = () => {
    mutation.mutate(undefined, {
      onSuccess: () => navigate('/'),
      onError: (error) => notifyError(error, 'Не удалось выйти из очереди'),
    });
  };

  return { leaveQueue, isPending: mutation.isPending };
};
