import { LeaveQueueButton } from '@features/leave-queue';
import { cn } from '@shared/lib';
import { Button, QuantityStepper } from '@ui';

import type { CtaAction } from '../lib/resolveCtaAction';
import styles from './OrderQueueCta.module.css';

const bem = cn('OrderQueueCta');

type Props = {
  productId: string;
  isQuantitySelectable: boolean;
  quantity: number;
  minQuantity: number;
  increase: () => void;
  decrease: () => void;
  action: CtaAction;
  isLoading: boolean;
  showLeave: boolean;
};

export const QueueCtaActions = ({
  productId,
  isQuantitySelectable,
  quantity,
  minQuantity,
  increase,
  decrease,
  action,
  isLoading,
  showLeave,
}: Props): React.JSX.Element => (
  <div className={styles[bem('actions')]}>
    {isQuantitySelectable && (
      <QuantityStepper
        disabled={isLoading}
        min={minQuantity}
        onDecrease={decrease}
        onIncrease={increase}
        value={quantity}
      />
    )}
    <Button
      className={styles[bem('button')]}
      disabled={action.disabled}
      loading={isLoading}
      onClick={action.run}
      size="large"
      variant="primary"
    >
      {action.label}
    </Button>
    {showLeave && <LeaveQueueButton productId={productId} />}
  </div>
);
