import { Outlet } from 'react-router-dom';

import { useUserStore } from '@entities/user';
import { cn } from '@shared/lib';
import { Header } from '@widgets/header';

import styles from './AppLayout.module.css';

const bem = cn('AppLayout');

export const AppLayout = (): React.JSX.Element => {
  const userId = useUserStore.use.userId();

  return (
    <div className={styles[bem()]} data-user-id={userId}>
      <Header />
      <Outlet />
    </div>
  );
};
