import type { Product } from '@entities/product';
import { cn } from '@shared/lib';
import { Card } from '@ui';

import { useJoinQueueProductCard } from '../model/useJoinQueueProductCard';
import styles from './JoinQueueProductCard.module.css';

const bem = cn('JoinQueueProductCard');

type Props = {
  product: Product;
};

export const JoinQueueProductCard = ({ product }: Props): React.JSX.Element => {
  const { isQueued, openProduct } = useJoinQueueProductCard(product);

  return (
    <Card
      cover={<img alt={product.title} src={product.image} />}
      description={product.description}
      onClick={openProduct}
      title={product.title}
    >
      <p className={styles[bem('price')]}>{product.price.toLocaleString('ru-RU')} ₽</p>
      {isQueued && <p className={styles[bem('queue-status')]}>Вы стоите в очереди</p>}
    </Card>
  );
};
