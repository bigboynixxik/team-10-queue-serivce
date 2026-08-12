import { Outlet } from 'react-router-dom';

import { useUserStore } from '@entities/user';
import { useMyQueuesLiveUpdates } from '@features/my-queues';
import { cn } from '@shared/lib';
import { Header } from '@widgets/header';

import styles from './AppLayout.module.css';

const bem = cn('AppLayout');

export const AppLayout = (): React.JSX.Element => {
  const userId = useUserStore.use.userId();
  const role = useUserStore.use.role();

  useMyQueuesLiveUpdates(role === 'buyer' ? userId : '');

  return (
    <div className={styles[bem()]} data-user-id={userId}>
      <Header />
      <Outlet />
    </div>
  );
};
