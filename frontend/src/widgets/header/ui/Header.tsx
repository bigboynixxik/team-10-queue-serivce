import avitoLogo from '@shared/assets/Avito.svg';
import { queueQueries } from '@entities/queue';
import { LeaveQueueButton } from '@features/leave-queue';
import { cn } from '@shared/lib';
import { useQuery } from '@tanstack/react-query';
import { useParams } from 'react-router-dom';

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
  const { productId } = useParams();

  return (
    <header className={styles[bem()]}>
      <img alt="Авито" className={styles[bem('logo')]} src={avitoLogo} />
      {productId && <QueueLeaveAction productId={productId} />}
    </header>
  );
};
