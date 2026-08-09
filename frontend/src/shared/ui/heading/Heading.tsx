import type { PropsWithChildren } from 'react';

import { cn } from '@shared/lib';

import styles from './Heading.module.css';

const bem = cn('Heading');

type Props = PropsWithChildren<{
  level?: 1 | 2 | 3;
}>;

export const Heading = ({ children, level = 1 }: Props): React.JSX.Element => {
  const Tag = `h${level}` as const;

  return (
    <Tag
      className={[styles[bem()], styles[bem(undefined, { [`level-${level}`]: true })]].join(' ')}
    >
      {children}
    </Tag>
  );
};
