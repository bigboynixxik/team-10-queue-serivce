import { useMutation } from '@tanstack/react-query';

import { type AcceptOfferPayload, queueMutations } from '@entities/queue';
import { useErrorNotifier } from '@shared/lib';

export const useOfferActions = (productId: string) => {
  const notifyError = useErrorNotifier();
  const acceptMutation = useMutation(queueMutations.acceptOffer(productId));
  const declineMutation = useMutation(queueMutations.declineOffer(productId));

  const accept = (payload: AcceptOfferPayload) => {
    acceptMutation.mutate(payload, {
      onError: (error) => notifyError(error, 'Не удалось принять предложение'),
    });
  };

  const decline = () => {
    declineMutation.mutate(undefined, {
      onError: (error) => notifyError(error, 'Не удалось отказаться от предложения'),
    });
  };

  return {
    accept,
    decline,
    isPending: acceptMutation.isPending || declineMutation.isPending,
  };
};
