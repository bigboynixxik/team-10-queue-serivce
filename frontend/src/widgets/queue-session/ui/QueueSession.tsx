import { QueueStatusView, useQueueStatus } from '@features/queue-status';
import { cn } from '@shared/lib';
import { Alert, Card, Heading, Spinner } from '@ui';
import styles from './QueueSession.module.css';
import { QueueSessionActions } from './QueueSessionActions';

const bem = cn('QueueSession');

type Props = {
  productId: string;
};

export const QueueSession = ({ productId }: Props): React.JSX.Element => {
  const { membership, secondsLeft, isPending, isError } = useQueueStatus(productId);

  if (isPending) return <Spinner size="large" />;
  if (isError || !membership) return <Alert title="Очередь не найдена" variant="error" />;

  return (
    <Card className={styles[bem()]}>
      <Heading>Статус очереди</Heading>
      <QueueStatusView membership={membership} secondsLeft={secondsLeft} />
      <QueueSessionActions productId={productId} membership={membership} />
    </Card>
  );
};
