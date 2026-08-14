import { Button, Modal } from '@ui';

import { useLeaveQueueButton } from '../model/useLeaveQueueButton';

type Props = {
  productId: string;
};

export const LeaveQueueButton = ({ productId }: Props): React.JSX.Element => {
  const { isOpen, open, close, confirmLeave, isPending } = useLeaveQueueButton(productId);

  return (
    <>
      <Button loading={isPending} onClick={open} size="large" variant="danger">
        Выйти из очереди
      </Button>
      <Modal
        description="Вы потеряете место в очереди и доступ к дефицитному товару. Вернуться на то же место уже не получится."
        onClose={close}
        open={isOpen}
        title="Вы точно хотите выйти из очереди?"
      >
        <Button onClick={close} variant="secondary">
          Остаться
        </Button>
        <Button loading={isPending} onClick={confirmLeave} variant="danger">
          Выйти
        </Button>
      </Modal>
    </>
  );
};
