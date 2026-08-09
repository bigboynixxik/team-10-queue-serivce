import type { PropsWithChildren, ReactNode } from 'react';

import { cn } from '@shared/lib';

import styles from './Card.module.css';

const bem = cn('Card');

type Props = PropsWithChildren<{
  className?: string;
  cover?: ReactNode;
  title?: string;
  description?: string;
  onClick?: () => void;
}>;

export const Card = ({
  children,
  className,
  cover,
  title,
  description,
  onClick,
}: Props): React.JSX.Element => (
  <article
    className={[styles[bem()], className].filter(Boolean).join(' ')}
    onClick={onClick}
    onKeyDown={(event) => {
      if (onClick && (event.key === 'Enter' || event.key === ' ')) {
        event.preventDefault();
        onClick();
      }
    }}
    tabIndex={onClick ? 0 : undefined}
  >
    {cover && <div className={styles[bem('cover')]}>{cover}</div>}
    <div className={styles[bem('content')]}>
      {title && <h2 className={styles[bem('title')]}>{title}</h2>}
      {description && <p className={styles[bem('description')]}>{description}</p>}
      {children}
    </div>
  </article>
);
