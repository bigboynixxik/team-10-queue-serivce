import type { Nullable } from '@shared/model';
import { Button, InputNumber, Space } from '@ui';
import { useState } from 'react';

import { useOfferActions } from '../model/useOfferActions';

type Props = {
  productId: string;
  availableQuantity: number;
};

export const OfferActions = ({ productId, availableQuantity }: Props) => {
  const [quantity, setQuantity] = useState(availableQuantity);
  const { accept, decline, isPending } = useOfferActions(productId);

  return (
    <Space wrap>
      <InputNumber
        min={1}
        max={availableQuantity}
        value={quantity}
        onChange={(value: Nullable<number>) => setQuantity(value ?? 1)}
      />
      <Button type="primary" loading={isPending} onClick={() => accept({ quantity })}>
        Принять предложение
      </Button>
      <Button danger loading={isPending} onClick={() => decline()}>
        Отказаться
      </Button>
    </Space>
  );
};
