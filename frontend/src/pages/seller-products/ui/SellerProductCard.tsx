import type { Product } from '@entities/product';
import { cn } from '@shared/lib';
import { Card } from '@ui';

import { useSellerProductCard } from '../model/useSellerProductCard';
import styles from './SellerProductCard.module.css';

const bem = cn('SellerProductCard');

type Props = {
  product: Product;
};

export const SellerProductCard = ({ product }: Props): React.JSX.Element => {
  const { openProduct } = useSellerProductCard(product);

  return (
    <Card
      cover={<img alt={product.title} src={product.image} />}
      description={product.description}
      onClick={openProduct}
      title={product.title}
    >
      <p className={styles[bem('price')]}>{product.price.toLocaleString('ru-RU')} ₽</p>
    </Card>
  );
};
