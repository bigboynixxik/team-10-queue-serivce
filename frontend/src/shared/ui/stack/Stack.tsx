import type { CSSProperties, PropsWithChildren } from 'react';

import { cn } from '@shared/lib';

import styles from './Stack.module.css';

const bem = cn('Stack');

type Props = PropsWithChildren<{
  className?: string;
  gap?: number;
  wrap?: boolean;
  compact?: boolean;
  block?: boolean;
}>;

export const Stack = ({
  children,
  className,
  gap = 12,
  wrap = false,
  compact = false,
  block = false,
}: Props): React.JSX.Element => (
  <div
    className={[
      styles[bem()],
      styles[bem(undefined, { wrap })],
      styles[bem(undefined, { compact })],
      styles[bem(undefined, { block })],
      className,
    ]
      .filter(Boolean)
      .join(' ')}
    style={{ '--stack-gap': `${gap}px` } as CSSProperties}
  >
    {children}
  </div>
);
