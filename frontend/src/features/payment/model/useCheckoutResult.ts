import { useQueryClient } from '@tanstack/react-query';
import { useEffect } from 'react';

import { queueQueries } from '@entities/queue';
import { CHECKOUT_BASE_URL } from '@shared/config';

import { isPaymentSucceeded } from './isPaymentSucceeded';

const checkoutOrigin = new URL(CHECKOUT_BASE_URL, window.location.href).origin;

/** postmessage от checkout только сигнал перезапросить статус */
export const useCheckoutResult = (productId: string): void => {
  const queryClient = useQueryClient();

  useEffect(() => {
    const onMessage = (event: MessageEvent) => {
      if (event.origin !== checkoutOrigin || !isPaymentSucceeded(event.data)) return;

      queryClient.invalidateQueries({ queryKey: queueQueries.me(productId).queryKey });
    };

    window.addEventListener('message', onMessage);

    return () => window.removeEventListener('message', onMessage);
  }, [productId, queryClient]);
};
