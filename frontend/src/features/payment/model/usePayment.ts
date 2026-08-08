import { rightsMutations } from '@entities/queue';
import { APP_BASENAME, CHECKOUT_BASE_URL } from '@shared/config';
import { useErrorNotifier } from '@shared/lib';
import { useMutation } from '@tanstack/react-query';

const buildCheckoutUrl = (productId: string, token: string): string => {
  const checkoutUrl = new URL('/checkout', CHECKOUT_BASE_URL);

  checkoutUrl.searchParams.set('token', token);
  checkoutUrl.searchParams.set('product_id', productId);
  checkoutUrl.searchParams.set(
    'return_url',
    `${window.location.origin}${APP_BASENAME}/payment-success`,
  );

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
        window.open(buildCheckoutUrl(productId, token), '_blank', 'noopener,noreferrer');
      },
      onError: (error) => notifyError(error, 'Не удалось перейти к оплате'),
    });
  };

  return { pay, isPending: mutation.isPending };
};
