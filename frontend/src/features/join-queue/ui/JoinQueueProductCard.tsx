import type { Product } from '@entities/product';
import { queueQueries } from '@entities/queue';
import { cn } from '@shared/lib';
import { useQuery } from '@tanstack/react-query';
import { Card } from '@ui';

import { useJoinQueueForm } from '../model/useJoinQueueForm';

import styles from './JoinQueueProductCard.module.css';

const bem = cn('JoinQueueProductCard');

type Props = {
  product: Product;
};

export const JoinQueueProductCard = ({ product }: Props): React.JSX.Element => {
  const { submit } = useJoinQueueForm(product);
  const { data: membership } = useQuery(queueQueries.me(product.id));
  const isQueued = membership?.status === 'QUEUED';

  return (
    <Card
      cover={<img alt={product.title} src={product.image} />}
      description={product.description}
      onClick={isQueued ? undefined : submit}
      title={product.title}
    >
      <p className={styles[bem('price')]}>{product.price.toLocaleString('ru-RU')} ₽</p>
      {isQueued && <p className={styles[bem('queue-status')]}>Вы стоите в очереди</p>}
    </Card>
  );
};
