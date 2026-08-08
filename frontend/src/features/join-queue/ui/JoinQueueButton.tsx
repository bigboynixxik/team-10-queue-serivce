import { Button } from '@ui';
import type { MouseEvent } from 'react';

type Props = {
  onJoin: () => void;
  isPending: boolean;
};

export const JoinQueueButton = ({ onJoin, isPending }: Props): React.JSX.Element => {
  const handleClick = (event: MouseEvent<HTMLButtonElement>) => {
    event.stopPropagation();
    onJoin();
  };

  return (
    <Button loading={isPending} onClick={handleClick} variant="primary">
      Встать в очередь
    </Button>
  );
};
