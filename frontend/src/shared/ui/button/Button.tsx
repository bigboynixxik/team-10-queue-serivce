import type { ButtonHTMLAttributes, PropsWithChildren } from 'react';

import { cn } from '@shared/lib';

import styles from './Button.module.css';

const bem = cn('Button');

type Props = PropsWithChildren<
  ButtonHTMLAttributes<HTMLButtonElement> & {
    variant?: 'primary' | 'danger' | 'secondary';
    size?: 'medium' | 'large';
    loading?: boolean;
  }
>;

export const Button = ({
  children,
  className,
  variant = 'secondary',
  size = 'medium',
  loading = false,
  disabled,
  type = 'button',
  ...props
}: Props): React.JSX.Element => (
  <button
    {...props}
    className={[
      styles[bem()],
      styles[bem(undefined, { [variant]: true })],
      styles[bem(undefined, { [size]: true })],
      className,
    ]
      .filter(Boolean)
      .join(' ')}
    disabled={disabled || loading}
    type={type}
  >
    {loading && <span aria-hidden className={styles[bem('spinner')]} />}
    {children}
  </button>
);
