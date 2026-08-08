import type { Membership } from '@entities/queue';
import { LeaveQueueButton } from '@features/leave-queue';
import { OfferActions } from '@features/offer-actions';
import { PaymentButton } from '@features/payment';
import { assertNever } from '@shared/lib';
import type { Nullable } from '@shared/model';
import { Stack } from '@ui';
import { Link } from 'react-router-dom';

type Props = {
  productId: string;
  membership: Membership;
  secondsLeft: Nullable<number>;
};

export const QueueSessionActions = ({ productId, membership, secondsLeft }: Props) => {
  switch (membership.status) {
    case 'QUEUED':
      return <LeaveQueueButton productId={productId} />;
    case 'OFFER_PENDING':
      return membership.available_quantity ? (
        <OfferActions productId={productId} availableQuantity={membership.available_quantity} />
      ) : null;
    case 'RIGHT_ACTIVE':
      return (
        <Stack wrap>
          <PaymentButton productId={productId} secondsLeft={secondsLeft} token={membership.token} />
          <LeaveQueueButton productId={productId} />
        </Stack>
      );
    case 'DECLINED':
    case 'PURCHASED':
    case 'SOLD_OUT':
      return <Link to="/">Вернуться к товарам</Link>;
    default:
      return assertNever(membership.status);
  }
};
