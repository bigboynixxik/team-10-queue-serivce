import avitoLogo from '@shared/assets/Avito.svg';
import { cn } from '@shared/lib';

import styles from './Header.module.css';

const bem = cn('Header');

export const Header = (): React.JSX.Element => (
  <header className={styles[bem()]}>
    <img alt="Авито" className={styles[bem('logo')]} src={avitoLogo} />
  </header>
);
