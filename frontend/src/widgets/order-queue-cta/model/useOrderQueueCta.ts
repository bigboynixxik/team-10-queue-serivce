import { useQuery } from '@tanstack/react-query';
import { useEffect, useRef } from 'react';
import { useNavigate } from 'react-router-dom';

import type { Product } from '@entities/product';
import { queueQueries } from '@entities/queue';
import { useUserStore } from '@entities/user';
import { useJoinQueueForm } from '@features/join-queue';
import { useCheckoutResult, usePayment } from '@features/payment';
import { useQueueStatus } from '@features/queue-status';
import { appPath } from '@shared/config';

import { resolveCtaAction } from '../lib/resolveCtaAction';

export const useOrderQueueCta = (product: Product) => {
  const navigate = useNavigate();
  const userId = useUserStore.use.userId();
  const { membership, secondsLeft, isPending } = useQueueStatus(product.id);
  const { data: queues = [] } = useQuery(queueQueries.allForUser(userId));
  const {
    quantity,
    minQuantity,
    increase,
    decrease,
    submit,
    isPending: isJoining,
  } = useJoinQueueForm(product);
  const { pay, isPending: isPaying } = usePayment(product.id, membership?.token);
  const previousStatus = useRef(membership?.status);

  useCheckoutResult(product.id);

  useEffect(() => {
    // редирект только при переходе в purchased
    if (
      membership?.status === 'PURCHASED' &&
      previousStatus.current !== undefined &&
      previousStatus.current !== 'PURCHASED'
    ) {
      navigate(appPath('/payment-success'));
    }

    previousStatus.current = membership?.status;
  }, [membership?.status, navigate]);

  const status = membership?.status;
  const isQueued = status === 'QUEUED';
  const isPayable = status === 'RIGHT_ACTIVE';
  const isSoldOut = status === 'SOLD_OUT';
  const queue = queues.find((item) => item.product_id === product.id);

  // при offer_pending вместо cta отвечаем на оффер
  const offeredQuantity = status === 'OFFER_PENDING' ? membership?.available_quantity : undefined;

  // после входа количество уже зафиксировано на бэке
  const isQuantitySelectable = !isQueued && !isPayable && !isSoldOut;

  const action = resolveCtaAction({
    isPayable,
    isQueued,
    isSoldOut,
    pay,
    submit,
    goHome: () => navigate(appPath()),
  });

  const showJoinHints = offeredQuantity === undefined && !isQueued && !isPayable && !isSoldOut;

  return {
    productId: product.id,
    isQueued,
    isPayable,
    isSoldOut,
    offeredQuantity,
    requestedQuantity: membership?.quantity,
    isQuantitySelectable,
    queue,
    secondsLeft,
    quantity,
    minQuantity,
    increase,
    decrease,
    action,
    showLeave: isQueued || isPayable,
    showJoinHints,
    isLoading: isPending || isJoining || isPaying,
  };
};
