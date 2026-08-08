import { rightsMutations } from '@entities/queue';
import { useErrorNotifier } from '@shared/lib';
import { useMutation } from '@tanstack/react-query';

import { buildCheckoutUrl } from './buildCheckoutUrl';

export const usePayment = (productId: string, token?: string) => {
  const notifyError = useErrorNotifier();
  const mutation = useMutation(rightsMutations.validate());

  const pay = () => {
    if (!token) {
      notifyError(new Error('Право на покупку не найдено'));
      return;
    }

    mutation.mutate(token, {
      onSuccess: () => {
        // Without an opener the checkout tab is not script-closable, so it could
        // not close itself after the payment.
        window.open(buildCheckoutUrl(productId, token), 'avito-checkout');
      },
      onError: (error) => notifyError(error, 'Не удалось перейти к оплате'),
    });
  };

  return { pay, isPending: mutation.isPending };
};
