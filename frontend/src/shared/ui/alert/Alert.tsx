import { cn } from '@shared/lib';

import styles from './Alert.module.css';

const bem = cn('Alert');

type Props = {
  variant?: 'error' | 'info' | 'success';
  title: string;
  description?: string;
  onClose?: () => void;
};

export const Alert = ({
  variant = 'info',
  title,
  description,
  onClose,
}: Props): React.JSX.Element => (
  <div
    className={[
      styles[bem()],
      styles[bem(undefined, { [variant]: true, dismissible: Boolean(onClose) })],
    ].join(' ')}
    role="alert"
  >
    <strong className={styles[bem('title')]}>{title}</strong>
    {description && <p className={styles[bem('description')]}>{description}</p>}
    {onClose && (
      <button
        aria-label="Закрыть уведомление"
        className={styles[bem('close')]}
        onClick={onClose}
        type="button"
      >
        ×
      </button>
    )}
  </div>
);
