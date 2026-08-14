import type { UserQueue } from '@entities/queue';
import { cn } from '@shared/lib';

import styles from './MyQueuesMenu.module.css';

const bem = cn('MyQueuesMenu');

type Props = {
  queue: UserQueue;
  title: string;
  onOpen: (productId: string) => void;
};

export const MyQueueItem = ({ queue, title, onOpen }: Props): React.JSX.Element => (
  <button
    className={styles[bem('queue')]}
    onClick={() => onOpen(queue.product_id)}
    role="menuitem"
    type="button"
  >
    <span className={styles[bem('queue-title')]}>{title}</span>
    {queue.status === 'QUEUED' && queue.position !== undefined && (
      <span className={styles[bem('queue-position')]}>Место в очереди: {queue.position}</span>
    )}
  </button>
);
