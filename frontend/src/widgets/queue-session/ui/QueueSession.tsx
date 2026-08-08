import { queueQueries } from '@entities/queue';
import { useUserStore } from '@entities/user';
import { useCheckoutResult } from '@features/payment';
import { QueueStatusView, useQueueStatus } from '@features/queue-status';
import { cn } from '@shared/lib';
import { useQuery } from '@tanstack/react-query';
import { Alert, Spinner } from '@ui';
import { useEffect, useRef } from 'react';
import { useNavigate } from 'react-router-dom';
import styles from './QueueSession.module.css';
import { QueueSessionActions } from './QueueSessionActions';

const bem = cn('QueueSession');

type Props = {
  productId: string;
};

export const QueueSession = ({ productId }: Props): React.JSX.Element => {
  const { membership, secondsLeft, isPending, isError } = useQueueStatus(productId);
  const userId = useUserStore.use.userId();
  const { data: userQueues = [] } = useQuery(queueQueries.allForUser(userId));
  const navigate = useNavigate();
  const previousStatus = useRef(membership?.status);
  const queue = userQueues.find((item) => item.product_id === productId);

  useCheckoutResult(productId);

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

  if (isPending) return <Spinner size="large" />;
  if (isError || !membership) return <Alert title="Очередь не найдена" variant="error" />;

  return (
    <section className={styles[bem()]}>
      <QueueStatusView
        etaSeconds={queue?.eta_seconds}
        membership={membership}
        position={queue?.position}
      />
      <QueueSessionActions
        membership={membership}
        productId={productId}
        secondsLeft={secondsLeft}
      />
    </section>
  );
};
