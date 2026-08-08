import { cn } from '@shared/lib';

import styles from './Spinner.module.css';

const bem = cn('Spinner');

type Props = {
  size?: 'small' | 'large';
};

export const Spinner = ({ size = 'small' }: Props): React.JSX.Element => (
  <span
    aria-label="Загрузка"
    className={[styles[bem()], styles[bem(undefined, { [size]: true })]].join(' ')}
    role="status"
  />
);
