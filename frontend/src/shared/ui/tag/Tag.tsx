import type { PropsWithChildren } from 'react';

import { cn } from '@shared/lib';

import styles from './Tag.module.css';

const bem = cn('Tag');

type Props = PropsWithChildren<{
  variant?: 'success' | 'neutral';
}>;

export const Tag = ({ children, variant = 'neutral' }: Props): React.JSX.Element => (
  <span className={[styles[bem()], styles[bem(undefined, { [variant]: true })]].join(' ')}>
    {children}
  </span>
);
