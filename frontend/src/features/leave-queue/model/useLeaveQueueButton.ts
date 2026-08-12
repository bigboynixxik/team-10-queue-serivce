import { useModal } from '@ui';

import { useLeaveQueue } from './useLeaveQueue';

export const useLeaveQueueButton = (productId: string) => {
  const { leaveQueue, isPending } = useLeaveQueue(productId);
  const { isOpen, open, close } = useModal();

  const confirmLeave = () => {
    leaveQueue();
    close();
  };

  return {
    isOpen,
    open,
    close,
    confirmLeave,
    isPending,
  };
};
