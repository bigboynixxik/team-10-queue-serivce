import { queueQueries } from '@entities/queue';
import { LeaveQueueButton } from '@features/leave-queue';
import { MyQueuesMenu } from '@features/my-queues';
import avitoLogo from '@shared/assets/Avito.svg';
import { cn } from '@shared/lib';
import { useQuery } from '@tanstack/react-query';
import { useMatch } from 'react-router-dom';

import styles from './Header.module.css';

const bem = cn('Header');

type QueueLeaveActionProps = {
  productId: string;
};

const QueueLeaveAction = ({ productId }: QueueLeaveActionProps): React.JSX.Element | null => {
  const { data: membership } = useQuery(queueQueries.me(productId));
  const canLeaveQueue = membership?.status === 'QUEUED' || membership?.status === 'RIGHT_ACTIVE';

  return canLeaveQueue ? <LeaveQueueButton productId={productId} /> : null;
};

export const Header = (): React.JSX.Element => {
  const productId = useMatch('/queue/:productId')?.params.productId;

  return (
    <header className={styles[bem()]}>
      <img alt="Авито" className={styles[bem('logo')]} src={avitoLogo} />
      {productId ? <QueueLeaveAction productId={productId} /> : <MyQueuesMenu />}
    </header>
  );
};
