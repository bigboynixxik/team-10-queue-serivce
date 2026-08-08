import type { Product } from '@entities/product';
import { Button, NumberInput, Stack } from '@ui';

import { useJoinQueueForm } from '../model/useJoinQueueForm';

type Props = {
  product: Product;
};

export const JoinQueueButton = ({ product }: Props): React.JSX.Element => {
  const { quantity, setQuantity, submit, isPending } = useJoinQueueForm(product);

  return (
    <Stack block compact>
      <NumberInput
        aria-label="Количество товара"
        min={1}
        value={quantity}
        onValueChange={setQuantity}
      />
      <Button loading={isPending} onClick={submit} variant="primary">
        Встать в очередь
      </Button>
    </Stack>
  );
};
