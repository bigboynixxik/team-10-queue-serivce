import type { Product } from '@entities/product';
import type { Nullable } from '@shared/model';
import { Button, InputNumber, Space } from '@ui';
import { useState } from 'react';

import { useJoinQueue } from '../model/useJoinQueue';

type Props = {
  product: Product;
};

export const JoinQueueButton = ({ product }: Props) => {
  const [quantity, setQuantity] = useState(1);
  const { join, isPending } = useJoinQueue(product);

  return (
    <Space.Compact block>
      <InputNumber
        aria-label="Количество товара"
        min={1}
        value={quantity}
        onChange={(value: Nullable<number>) => setQuantity(value ?? 1)}
      />
      <Button type="primary" loading={isPending} onClick={() => join(quantity)}>
        Встать в очередь
      </Button>
    </Space.Compact>
  );
};
