import type { Product } from '@entities/product';
import { OfferActions } from '@features/offer-actions';
import { cn } from '@shared/lib';

import { useOrderQueueCta } from '../model/useOrderQueueCta';
import styles from './OrderQueueCta.module.css';
import { QueueCtaActions } from './QueueCtaActions';
import { QueueStatusInfo } from './QueueStatusInfo';

const bem = cn('OrderQueueCta');

type Props = {
  product: Product;
};

export const OrderQueueCta = ({ product }: Props): React.JSX.Element => {
  const {
    productId,
    isQueued,
    isPayable,
    isSoldOut,
    offeredQuantity,
    requestedQuantity,
    isQuantitySelectable,
    queue,
    secondsLeft,
    quantity,
    minQuantity,
    increase,
    decrease,
    action,
    showLeave,
    isLoading,
  } = useOrderQueueCta(product);

  return (
    <div className={styles[bem()]}>
      <QueueStatusInfo
        etaSeconds={queue?.eta_seconds}
        isPayable={isPayable}
        isQueued={isQueued}
        isSoldOut={isSoldOut}
        position={queue?.position}
        secondsLeft={secondsLeft}
      />
      {offeredQuantity === undefined ? (
        <QueueCtaActions
          action={action}
          decrease={decrease}
          increase={increase}
          isLoading={isLoading}
          isQuantitySelectable={isQuantitySelectable}
          minQuantity={minQuantity}
          productId={productId}
          quantity={quantity}
          showLeave={showLeave}
        />
      ) : (
        <OfferActions
          availableQuantity={offeredQuantity}
          productId={productId}
          requestedQuantity={requestedQuantity}
        />
      )}
    </div>
  );
};
