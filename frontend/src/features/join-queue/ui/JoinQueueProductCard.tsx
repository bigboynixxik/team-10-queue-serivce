import type { Product } from '@entities/product';
import { cn } from '@shared/lib';
import { Card } from '@ui';

import { useJoinQueueForm } from '../model/useJoinQueueForm';

import styles from './JoinQueueProductCard.module.css';

const bem = cn('JoinQueueProductCard');

type Props = {
  product: Product;
};

export const JoinQueueProductCard = ({ product }: Props): React.JSX.Element => {
  const { submit } = useJoinQueueForm(product);

  return (
    <Card
      cover={<img alt={product.title} src={product.image} />}
      description={product.description}
      onClick={submit}
      title={product.title}
    >
      <p className={styles[bem('price')]}>{product.price.toLocaleString('ru-RU')} ₽</p>
    </Card>
  );
};
