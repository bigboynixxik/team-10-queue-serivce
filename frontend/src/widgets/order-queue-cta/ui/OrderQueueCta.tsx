import type { Product } from '@entities/product';
import { queueQueries } from '@entities/queue';
import { useUserStore } from '@entities/user';
import { useJoinQueueForm } from '@features/join-queue';
import { usePayment } from '@features/payment';
import { useQueueStatus } from '@features/queue-status';
import { cn } from '@shared/lib';
import { useQuery } from '@tanstack/react-query';
import { Button } from '@ui';
import { useNavigate } from 'react-router-dom';

import styles from './OrderQueueCta.module.css';

const bem = cn('OrderQueueCta');

type Props = {
  product: Product;
};

const formatTimeLeft = (secondsLeft: number): string => {
  const minutes = Math.floor(secondsLeft / 60);
  const seconds = secondsLeft % 60;

  return `${String(minutes).padStart(2, '0')}:${String(seconds).padStart(2, '0')}`;
};

export const OrderQueueCta = ({ product }: Props): React.JSX.Element => {
  const navigate = useNavigate();
  const userId = useUserStore.use.userId();
  const { membership, secondsLeft, isPending } = useQueueStatus(product.id);
  const { data: queues = [] } = useQuery(queueQueries.allForUser(userId));
  const { submit, isPending: isJoining } = useJoinQueueForm(product);
  const { pay, isPending: isPaying } = usePayment(product.id, membership?.token);

  const isWaiting = membership?.status === 'QUEUED' || membership?.status === 'OFFER_PENDING';
  const isPayable = membership?.status === 'RIGHT_ACTIVE';
  const position = queues.find((queue) => queue.product_id === product.id)?.position;

  const action = (() => {
    if (isPayable) return { label: 'Оплатить товар', run: pay };
    if (isWaiting)
      return { label: 'Ожидания очереди', run: () => navigate(`/queue/${product.id}`) };

    return { label: 'Перейти в очередь', run: submit };
  })();

  return (
    <div className={styles[bem()]}>
      {isWaiting && position !== undefined && (
        <p className={styles[bem('position')]}>
          Место в очереди: <span className={styles[bem('position-value')]}>{position}</span>
        </p>
      )}
      {isPayable && secondsLeft !== null && (
        <p className={styles[bem('timer')]}>
          До конца оплаты:{' '}
          <span className={styles[bem('timer-value')]}>{formatTimeLeft(secondsLeft)}</span>
        </p>
      )}
      <Button
        className={styles[bem('button')]}
        loading={isPending || isJoining || isPaying}
        onClick={action.run}
        size="large"
        variant="primary"
      >
        {action.label}
      </Button>
    </div>
  );
};
