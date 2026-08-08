import type { Membership } from '@entities/queue';

import styles from './QueueStatusView.module.css';

type Props = {
  membership: Membership;
};

/**
 * Status changes are announced by the single SSE notifier, which sees every
 * queue of the user: a second toast from this screen would repeat the same news
 * for the product already open on it.
 */
export const QueueStatusView = ({ membership }: Props): React.JSX.Element | null => {
  if (membership.status === 'QUEUED') {
    return <p className={styles.QueueStatusView}>Ожидайте...</p>;
  }

  return null;
};
