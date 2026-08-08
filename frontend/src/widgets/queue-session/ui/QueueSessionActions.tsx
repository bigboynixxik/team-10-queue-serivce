import type { Membership } from '@entities/queue';
import { OfferActions } from '@features/offer-actions';
import { PaymentButton } from '@features/payment';
import { assertNever } from '@shared/lib';
import type { Nullable } from '@shared/model';
import { Button } from '@ui';
import { Link, useNavigate } from 'react-router-dom';

type Props = {
  productId: string;
  membership: Membership;
  secondsLeft: Nullable<number>;
};

const SoldOutActions = (): React.JSX.Element => {
  const navigate = useNavigate();

  return (
    <>
      <p>К сожалению товара больше нет, вы исключены из очереди</p>
      <Button onClick={() => navigate('/')} size="medium" variant="secondary">
        Вернуться к товарам
      </Button>
    </>
  );
};

export const QueueSessionActions = ({ productId, membership, secondsLeft }: Props) => {
  switch (membership.status) {
    case 'QUEUED':
      return null;
    case 'OFFER_PENDING':
      return membership.available_quantity ? (
        <OfferActions productId={productId} availableQuantity={membership.available_quantity} />
      ) : null;
    case 'RIGHT_ACTIVE':
      return <PaymentButton productId={productId} secondsLeft={secondsLeft} token={membership.token} />;
    case 'DECLINED':
      return null;
    case 'PURCHASED':
      return <Link to="/">Вернуться к товарам</Link>;
    case 'SOLD_OUT':
      return <SoldOutActions />;
    default:
      return assertNever(membership.status);
  }
};
