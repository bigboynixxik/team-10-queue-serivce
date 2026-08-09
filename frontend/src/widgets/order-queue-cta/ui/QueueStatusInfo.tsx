import { cn } from '@shared/lib';
import { Alert } from '@ui';

import { formatEta, formatTimeLeft } from '../lib/formatTime';
import styles from './OrderQueueCta.module.css';

const bem = cn('OrderQueueCta');

type Props = {
  isQueued: boolean;
  isPayable: boolean;
  isSoldOut: boolean;
  position?: number;
  etaSeconds?: number;
  secondsLeft: number | null;
};

export const QueueStatusInfo = ({
  isQueued,
  isPayable,
  isSoldOut,
  position,
  etaSeconds,
  secondsLeft,
}: Props): React.JSX.Element => (
  <>
    {isQueued && position !== undefined && (
      <p className={styles[bem('position')]}>
        Место в очереди: <span className={styles[bem('position-value')]}>{position}</span>
      </p>
    )}
    {isQueued && etaSeconds !== undefined && (
      <p className={styles[bem('eta')]}>
        Ориентировочное время ожидания: <strong>{formatEta(etaSeconds)}</strong>
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
  </>
);
