import type { ReactNode } from 'react';

import type { Product } from '@entities/product';
import { cn } from '@shared/lib';
import { Alert, Spinner } from '@ui';

import { useProductList } from '../model/useProductList';
import styles from './ProductList.module.css';

const bem = cn('ProductList');

type Props = {
  excludeId?: string;
  renderItem: (product: Product) => ReactNode;
};

export const ProductList = ({ excludeId, renderItem }: Props): React.JSX.Element => {
  const { data: products, isPending, isError } = useProductList();

  if (isPending) return <Spinner size="large" />;
  if (isError || !products) {
    return <Alert title="Не удалось загрузить товары" variant="error" />;
  }

  const items = excludeId ? products.filter((product) => product.id !== excludeId) : products;

  return <div className={styles[bem()]}>{items.map(renderItem)}</div>;
};
