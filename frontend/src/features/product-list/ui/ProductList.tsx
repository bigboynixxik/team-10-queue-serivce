import type { Product } from '@entities/product';
import { cn } from '@shared/lib';
import { Alert, Card, Spinner } from '@ui';
import type { ReactNode } from 'react';
import { useProductList } from '../model/useProductList';

import styles from './ProductList.module.css';

const bem = cn('ProductList');

type Props = {
  renderAction: (product: Product) => ReactNode;
};

export const ProductList = ({ renderAction }: Props): React.JSX.Element => {
  const { data: products, isPending, isError } = useProductList();

  if (isPending) return <Spinner size="large" />;
  if (isError || !products) {
    return <Alert title="Не удалось загрузить товары" variant="error" />;
  }

  return (
    <div className={styles[bem()]}>
      {products.map((product) => (
        <Card
          cover={<img alt={product.title} src={product.image} />}
          key={product.id}
          description={product.description}
          title={product.title}
        >
          <p className={styles[bem('price')]}>{product.price.toLocaleString('ru-RU')} ₽</p>
          {renderAction(product)}
        </Card>
      ))}
    </div>
  );
};
