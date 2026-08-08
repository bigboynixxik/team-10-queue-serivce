import { QueueStatusView, useQueueStatus } from '@features/queue-status';
import { Alert, Card, Spin, Typography } from '@ui';

import { QueueSessionActions } from './QueueSessionActions';

type Props = {
  productId: string;
};

export const QueueSession = ({ productId }: Props) => {
  const { membership, secondsLeft, isPending, isError } = useQueueStatus(productId);

  if (isPending) return <Spin size="large" />;
  if (isError || !membership) return <Alert type="error" message="Очередь не найдена" />;

  return (
    <Card className="queue-card">
      <Typography.Title level={1}>Статус очереди</Typography.Title>
      <QueueStatusView membership={membership} secondsLeft={secondsLeft} />
      <QueueSessionActions productId={productId} membership={membership} />
    </Card>
  );
};
