import type { InputHTMLAttributes } from 'react';

import { cn } from '@shared/lib';

import styles from './NumberInput.module.css';

const bem = cn('NumberInput');

type Props = Omit<InputHTMLAttributes<HTMLInputElement>, 'onChange' | 'type' | 'value'> & {
  value: number;
  onValueChange: (value: number | null) => void;
};

export const NumberInput = ({
  className,
  onValueChange,
  value,
  ...props
}: Props): React.JSX.Element => (
  <input
    {...props}
    className={[styles[bem()], className].filter(Boolean).join(' ')}
    onChange={(event) => {
      const nextValue = event.target.valueAsNumber;
      onValueChange(Number.isNaN(nextValue) ? null : nextValue);
    }}
    type="number"
    value={value}
  />
);
