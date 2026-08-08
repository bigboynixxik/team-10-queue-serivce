import { Button } from '@ui';

import { usePayment } from '../model/usePayment';

type Props = {
  productId: string;
  token?: string;
};

export const PaymentButton = ({ productId, token }: Props) => {
  const { pay, isPending } = usePayment(productId, token);

  return (
    <Button type="primary" size="large" loading={isPending} onClick={() => pay()}>
      Перейти к оплате
    </Button>
  );
};
