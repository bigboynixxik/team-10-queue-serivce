import type { ReactNode } from 'react';

import { cn } from '@shared/lib';

import styles from './DescriptionList.module.css';

const bem = cn('DescriptionList');

export type DescriptionItem = {
  label: string;
  value: ReactNode;
};

type Props = {
  items: DescriptionItem[];
};

export const DescriptionList = ({ items }: Props): React.JSX.Element => (
  <dl className={styles[bem()]}>
    {items.map(({ label, value }) => (
      <div className={styles[bem('item')]} key={label}>
        <dt className={styles[bem('label')]}>{label}</dt>
        <dd className={styles[bem('value')]}>{value}</dd>
      </div>
    ))}
  </dl>
);
