import { useMutation } from '@tanstack/react-query';

import { rightsMutations } from '@entities/queue';
import { useErrorNotifier } from '@shared/lib';

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
        // именованное окно нужно чтобы checkout закрыл себя
        window.open(buildCheckoutUrl(productId, token), 'avito-checkout');
      },
      onError: (error) => notifyError(error, 'Не удалось перейти к оплате'),
    });
  };

  return { pay, isPending: mutation.isPending };
};
