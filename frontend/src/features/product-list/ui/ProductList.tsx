import type { Product } from '@entities/product';
import { cn } from '@shared/lib';
import { Alert, Spinner } from '@ui';
import type { ReactNode } from 'react';
import { useProductList } from '../model/useProductList';

import styles from './ProductList.module.css';

const bem = cn('ProductList');

type Props = {
  renderItem: (product: Product) => ReactNode;
};

export const ProductList = ({ renderItem }: Props): React.JSX.Element => {
  const { data: products, isPending, isError } = useProductList();

  if (isPending) return <Spinner size="large" />;
  if (isError || !products) {
    return <Alert title="Не удалось загрузить товары" variant="error" />;
  }

  return <div className={styles[bem()]}>{products.map(renderItem)}</div>;
};
