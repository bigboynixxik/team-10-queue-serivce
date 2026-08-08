import { Button } from '@ui';

import { usePayment } from '../model/usePayment';

type Props = {
  productId: string;
  token?: string;
};

export const PaymentButton = ({ productId, token }: Props): React.JSX.Element => {
  const { pay, isPending } = usePayment(productId, token);

  return (
    <Button loading={isPending} onClick={pay} size="large" variant="primary">
      Перейти к оплате
    </Button>
  );
};
