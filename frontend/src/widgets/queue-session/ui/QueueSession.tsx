import { QueueStatusView, useQueueStatus } from '@features/queue-status';
import { productQueries } from '@entities/product';
import { cn } from '@shared/lib';
import { Alert, Spinner } from '@ui';
import { useQuery } from '@tanstack/react-query';
import styles from './QueueSession.module.css';
import { QueueSessionActions } from './QueueSessionActions';

const bem = cn('QueueSession');

type Props = {
  productId: string;
};

export const QueueSession = ({ productId }: Props): React.JSX.Element => {
  const { membership, secondsLeft, isPending, isError } = useQueueStatus(productId);
  const { data: product } = useQuery(productQueries.byId(productId));

  if (isPending) return <Spinner size="large" />;
  if (isError || !membership) return <Alert title="Очередь не найдена" variant="error" />;

  return (
    <section className={styles[bem()]}>
      {product && <QueueStatusView membership={membership} productTitle={product.title} />}
      <QueueSessionActions
        membership={membership}
        productId={productId}
        secondsLeft={secondsLeft}
      />
    </section>
  );
};
