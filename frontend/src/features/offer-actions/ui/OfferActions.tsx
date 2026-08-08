import { Button, NumberInput, Stack } from '@ui';

import { useOfferActionsForm } from '../model/useOfferActionsForm';

type Props = {
  productId: string;
  availableQuantity: number;
};

export const OfferActions = ({ productId, availableQuantity }: Props): React.JSX.Element => {
  const { quantity, setQuantity, accept, decline, isPending } = useOfferActionsForm(
    productId,
    availableQuantity,
  );

  return (
    <Stack wrap>
      <NumberInput min={1} max={availableQuantity} value={quantity} onValueChange={setQuantity} />
      <Button loading={isPending} onClick={accept} variant="primary">
        Принять предложение
      </Button>
      <Button loading={isPending} onClick={decline} variant="danger">
        Отказаться
      </Button>
    </Stack>
  );
};
