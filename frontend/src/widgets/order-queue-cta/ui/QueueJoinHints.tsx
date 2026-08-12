import { cn } from '@shared/lib';
import { Alert } from '@ui';

import { QUEUE_JOIN_HINTS } from '../lib/queueJoinHints';
import styles from './OrderQueueCta.module.css';

const bem = cn('OrderQueueCta');

export const QueueJoinHints = (): React.JSX.Element => (
  <div className={styles[bem('hints')]}>
    {QUEUE_JOIN_HINTS.map((hint) => (
      <Alert description={hint.description} key={hint.title} title={hint.title} variant="info" />
    ))}
  </div>
);
