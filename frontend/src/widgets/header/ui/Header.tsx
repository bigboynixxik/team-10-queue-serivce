import { Link } from 'react-router-dom';

import { useUserStore } from '@entities/user';
import { MyQueuesMenu } from '@features/my-queues';
import { RoleSelector } from '@features/switch-role';
import { appPath } from '@shared/config';
import { cn } from '@shared/lib';
import { avitoLogo } from '@ui';

import styles from './Header.module.css';

const bem = cn('Header');

export const Header = (): React.JSX.Element => {
  const role = useUserStore.use.role();

  return (
    <header className={styles[bem()]}>
      <Link to={appPath()}>
        <img alt="Авито" className={styles[bem('logo')]} src={avitoLogo} />
      </Link>
      <div className={styles[bem('actions')]}>
        <RoleSelector />
        {role === 'buyer' ? (
          <MyQueuesMenu />
        ) : (
          <Link className={styles[bem('seller-link')]} to={appPath('/seller/products')}>
            Мои товары
          </Link>
        )}
      </div>
    </header>
  );
};
