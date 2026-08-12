import { Link } from 'react-router-dom';

import { MyQueuesMenu } from '@features/my-queues';
import { cn } from '@shared/lib';
import { avitoLogo } from '@ui';

import styles from './Header.module.css';

const bem = cn('Header');

export const Header = (): React.JSX.Element => (
  <header className={styles[bem()]}>
    <Link to="/avito">
      <img alt="Авито" className={styles[bem('logo')]} src={avitoLogo} />
    </Link>
    <MyQueuesMenu />
  </header>
);
