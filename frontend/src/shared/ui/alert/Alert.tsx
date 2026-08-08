import { cn } from '@shared/lib';

import styles from './Alert.module.css';

const bem = cn('Alert');

type Props = {
  variant?: 'error' | 'info' | 'success';
  title: string;
  description?: string;
};

export const Alert = ({ variant = 'info', title, description }: Props): React.JSX.Element => (
  <div
    className={[styles[bem()], styles[bem(undefined, { [variant]: true })]].join(' ')}
    role="alert"
  >
    <strong className={styles[bem('title')]}>{title}</strong>
    {description && <p className={styles[bem('description')]}>{description}</p>}
  </div>
);
