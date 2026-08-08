import { queueQueries } from '@entities/queue';
import { CHECKOUT_BASE_URL } from '@shared/config';
import { useQueryClient } from '@tanstack/react-query';
import { useEffect } from 'react';

const checkoutOrigin = new URL(CHECKOUT_BASE_URL, window.location.href).origin;

const isPaymentSucceeded = (data: unknown): boolean => {
  if (typeof data !== 'object' || data === null) return false;

  const message = data as { source?: unknown; event?: unknown };

  return message.source === 'avito-checkout' && message.event === 'payment_succeeded';
};

/**
 * The checkout tab closes itself right after the payment and tells this tab
 * about it. The message is only a hint to refetch: the status still comes from
 * Queue Service, which learns about the purchase from the backend event.
 */
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
