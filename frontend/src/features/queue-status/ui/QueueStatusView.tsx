import type { Membership } from '@entities/queue';

import styles from './QueueStatusView.module.css';

type Props = {
  membership: Membership;
  position?: number;
  etaSeconds?: number;
};

const formatEta = (etaSeconds: number): string => {
  if (etaSeconds < 60) return 'менее минуты';

  const minutes = Math.floor(etaSeconds / 60);
  const seconds = etaSeconds % 60;

  return seconds ? `${minutes} мин. ${seconds} сек.` : `${minutes} мин.`;
};

export const QueueStatusView = ({
  membership,
  position,
  etaSeconds,
}: Props): React.JSX.Element | null => {
  if (membership.status === 'QUEUED') {
    return (
      <div className={styles.QueueStatusView}>
        {position !== undefined && (
          <>
            <p className={styles.QueueStatusView__position}>
              {position}
            </p>
            <p className={`${styles.QueueStatusView__text} ${styles.QueueStatusView__detail}`}>место</p>
          </>
        )}
        {etaSeconds !== undefined && (
          <p className={styles.QueueStatusView__text}>
            Ориентировочное время ожидания: <strong>{formatEta(etaSeconds)}</strong>
          </p>
        )}
      </div>
    );
  }

  return null;
};
