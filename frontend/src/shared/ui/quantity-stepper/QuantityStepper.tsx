import { cn } from '@shared/lib';

import styles from './QuantityStepper.module.css';

const bem = cn('QuantityStepper');

type Props = {
  value: number;
  min?: number;
  disabled?: boolean;
  onDecrease: () => void;
  onIncrease: () => void;
};

export const QuantityStepper = ({
  value,
  min = 1,
  disabled = false,
  onDecrease,
  onIncrease,
}: Props): React.JSX.Element => (
  <div className={styles[bem()]}>
    <button
      aria-label="Уменьшить количество"
      className={styles[bem('button')]}
      disabled={disabled || value <= min}
      onClick={onDecrease}
      type="button"
    >
      −
    </button>
    <span aria-live="polite" className={styles[bem('value')]}>
      {value}
    </span>
    <button
      aria-label="Увеличить количество"
      className={styles[bem('button')]}
      disabled={disabled}
      onClick={onIncrease}
      type="button"
    >
      +
    </button>
  </div>
);
