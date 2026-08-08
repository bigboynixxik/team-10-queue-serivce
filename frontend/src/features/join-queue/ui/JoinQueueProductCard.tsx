import type { Product } from '@entities/product';
import { queueQueries } from '@entities/queue';
import { cn } from '@shared/lib';
import { useQuery } from '@tanstack/react-query';
import { Card } from '@ui';
import { useNavigate } from 'react-router-dom';

import styles from './JoinQueueProductCard.module.css';

const bem = cn('JoinQueueProductCard');

type Props = {
  product: Product;
};

export const JoinQueueProductCard = ({ product }: Props): React.JSX.Element => {
  const navigate = useNavigate();
  const { data: membership } = useQuery(queueQueries.me(product.id));
  const isQueued = membership?.status === 'QUEUED';

  return (
    <Card
      cover={<img alt={product.title} src={product.image} />}
      description={product.description}
      onClick={() => navigate(`/order-info/${product.id}`)}
      title={product.title}
    >
      <p className={styles[bem('price')]}>{product.price.toLocaleString('ru-RU')} ₽</p>
      {isQueued && <p className={styles[bem('queue-status')]}>Вы стоите в очереди</p>}
    </Card>
  );
};
