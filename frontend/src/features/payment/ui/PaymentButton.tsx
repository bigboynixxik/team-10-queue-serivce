import { Button } from '@ui';
import type { Nullable } from '@shared/model';

import { usePayment } from '../model/usePayment';

import styles from './PaymentButton.module.css';

type Props = {
  productId: string;
  token?: string;
  secondsLeft: Nullable<number>;
};

const formatTimeLeft = (secondsLeft: number): string => {
  const minutes = Math.floor(secondsLeft / 60);
  const seconds = secondsLeft % 60;

  return `${String(minutes).padStart(2, '0')}:${String(seconds).padStart(2, '0')}`;
};

export const PaymentButton = ({
  productId,
  token,
  secondsLeft,
}: Props): React.JSX.Element => {
  const { pay, isPending } = usePayment(productId, token);

  return (
    <div className={styles.PaymentButton}>
      {secondsLeft !== null && (
        <span className={styles.PaymentButton__timer}>{formatTimeLeft(secondsLeft)}</span>
      )}
      <Button loading={isPending} onClick={pay} size="medium" variant="secondary">
        Перейти к оплате
      </Button>
    </div>
  );
};
