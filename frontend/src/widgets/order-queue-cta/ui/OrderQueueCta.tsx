import type { Product } from '@entities/product';
import { queueQueries } from '@entities/queue';
import { useUserStore } from '@entities/user';
import { useJoinQueueForm } from '@features/join-queue';
import { LeaveQueueButton } from '@features/leave-queue';
import { OfferActions } from '@features/offer-actions';
import { useCheckoutResult, usePayment } from '@features/payment';
import { useQueueStatus } from '@features/queue-status';
import { cn } from '@shared/lib';
import { useQuery } from '@tanstack/react-query';
import { Alert, Button, QuantityStepper } from '@ui';
import { useEffect, useRef } from 'react';
import { useNavigate } from 'react-router-dom';

import styles from './OrderQueueCta.module.css';

const bem = cn('OrderQueueCta');

type Props = {
  product: Product;
};

type CtaAction = {
  label: string;
  run: () => void;
  disabled?: boolean;
};

const formatTimeLeft = (secondsLeft: number): string => {
  const minutes = Math.floor(secondsLeft / 60);
  const seconds = secondsLeft % 60;

  return `${String(minutes).padStart(2, '0')}:${String(seconds).padStart(2, '0')}`;
};

const formatEta = (etaSeconds: number): string => {
  if (etaSeconds < 60) return 'менее минуты';

  const minutes = Math.floor(etaSeconds / 60);
  const seconds = etaSeconds % 60;

  return seconds ? `${minutes} мин. ${seconds} сек.` : `${minutes} мин.`;
};

export const OrderQueueCta = ({ product }: Props): React.JSX.Element => {
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
    if (
      membership?.status === 'PURCHASED' &&
      previousStatus.current !== undefined &&
      previousStatus.current !== 'PURCHASED'
    ) {
      navigate('/payment-success');
    }

    previousStatus.current = membership?.status;
  }, [membership?.status, navigate]);

  const status = membership?.status;
  const isQueued = status === 'QUEUED';
  const isPayable = status === 'RIGHT_ACTIVE';
  const isSoldOut = status === 'SOLD_OUT';
  const queue = queues.find((item) => item.product_id === product.id);

  // The offer replaces the whole call to action: until the user answers it, no
  // other action on this product is possible.
  const offeredQuantity = status === 'OFFER_PENDING' ? membership?.available_quantity : undefined;

  // Quantity is fixed once the user is in the queue: the backend locks it into the
  // membership, so the stepper only makes sense before joining.
  const isQuantitySelectable = !isQueued && !isPayable && !isSoldOut;

  const action: CtaAction = (() => {
    if (isPayable) return { label: 'Оплатить товар', run: pay };
    if (isQueued) return { label: 'Вы в очереди', run: () => {}, disabled: true };
    if (isSoldOut) return { label: 'Вернуться к товарам', run: () => navigate('/') };

    return { label: 'Перейти в очередь', run: submit };
  })();

  return (
    <div className={styles[bem()]}>
      {isQueued && queue?.position !== undefined && (
        <p className={styles[bem('position')]}>
          Место в очереди: <span className={styles[bem('position-value')]}>{queue.position}</span>
        </p>
      )}
      {isQueued && queue?.eta_seconds !== undefined && (
        <p className={styles[bem('eta')]}>
          Ориентировочное время ожидания: <strong>{formatEta(queue.eta_seconds)}</strong>
        </p>
      )}
      {isPayable && secondsLeft !== null && (
        <p className={styles[bem('timer')]}>
          До конца оплаты:{' '}
          <span className={styles[bem('timer-value')]}>{formatTimeLeft(secondsLeft)}</span>
        </p>
      )}
      {isSoldOut && (
        <Alert
          description="Попробуйте оформить заказ позже или вернитесь к другим товарам."
          title="Товар закончился"
          variant="error"
        />
      )}
      {offeredQuantity === undefined ? (
        <div className={styles[bem('actions')]}>
          {isQuantitySelectable && (
            <QuantityStepper
              disabled={isPending || isJoining}
              min={minQuantity}
              onDecrease={decrease}
              onIncrease={increase}
              value={quantity}
            />
          )}
          <Button
            className={styles[bem('button')]}
            disabled={action.disabled}
            loading={isPending || isJoining || isPaying}
            onClick={action.run}
            size="large"
            variant="primary"
          >
            {action.label}
          </Button>
          {(isQueued || isPayable) && <LeaveQueueButton productId={product.id} />}
        </div>
      ) : (
        <OfferActions
          availableQuantity={offeredQuantity}
          productId={product.id}
          requestedQuantity={membership?.quantity}
        />
      )}
    </div>
  );
};
