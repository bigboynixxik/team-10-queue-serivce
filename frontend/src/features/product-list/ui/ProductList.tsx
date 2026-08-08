import type { Product } from '@entities/product';
import { Alert, Card, Spin } from '@ui';
import type { ReactNode } from 'react';

import { useProductList } from '../model/useProductList';

type Props = {
  renderAction: (product: Product) => ReactNode;
};

export const ProductList = ({ renderAction }: Props) => {
  const { data: products, isPending, isError } = useProductList();

  if (isPending) return <Spin size="large" />;
  if (isError || !products) {
    return <Alert type="error" message="Не удалось загрузить товары" />;
  }

  return (
    <div className="product-grid">
      {products.map((product) => (
        <Card
          key={product.id}
          cover={<img src={product.image} alt={product.title} />}
          className="product-card"
        >
          <Card.Meta title={product.title} description={product.description} />
          <p className="product-price">{product.price.toLocaleString('ru-RU')} ₽</p>
          {renderAction(product)}
        </Card>
      ))}
    </div>
  );
};
