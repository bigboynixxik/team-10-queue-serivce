import type { Membership } from '@entities/queue';
import { OfferActions } from '@features/offer-actions';
import { PaymentButton } from '@features/payment';
import { assertNever } from '@shared/lib';
import { Link } from 'react-router-dom';

type Props = {
  productId: string;
  membership: Membership;
};

export const QueueSessionActions = ({ productId, membership }: Props) => {
  switch (membership.status) {
    case 'QUEUED':
      return null;
    case 'OFFER_PENDING':
      return membership.available_quantity ? (
        <OfferActions productId={productId} availableQuantity={membership.available_quantity} />
      ) : null;
    case 'RIGHT_ACTIVE':
      return <PaymentButton productId={productId} token={membership.token} />;
    case 'DECLINED':
    case 'PURCHASED':
    case 'SOLD_OUT':
      return <Link to="/">Вернуться к товарам</Link>;
    default:
      return assertNever(membership.status);
  }
};
