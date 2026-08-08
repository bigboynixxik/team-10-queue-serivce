import { rightsMutations } from '@entities/queue';
import { CHECKOUT_BASE_URL } from '@shared/config';
import { useErrorNotifier } from '@shared/lib';
import { useMutation } from '@tanstack/react-query';

const buildCheckoutUrl = (productId: string, token: string): string => {
  const checkoutUrl = new URL('/checkout', CHECKOUT_BASE_URL);

  checkoutUrl.searchParams.set('token', token);
  checkoutUrl.searchParams.set('product_id', productId);

  return checkoutUrl.toString();
};

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
