import { Button } from '@ui';

import { useLeaveQueue } from '../model/useLeaveQueue';

type Props = {
  productId: string;
};

export const LeaveQueueButton = ({ productId }: Props): React.JSX.Element => {
  const { leaveQueue, isPending } = useLeaveQueue(productId);

  return (
    <Button loading={isPending} onClick={leaveQueue} size="large" variant="danger">
      Выйти из очереди
    </Button>
  );
};
