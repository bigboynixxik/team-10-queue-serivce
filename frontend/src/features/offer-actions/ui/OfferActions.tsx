import { cn } from '@shared/lib';
import { Alert, Button } from '@ui';

import { describeShortage } from '../lib/describeShortage';
import { useOfferActions } from '../model/useOfferActions';
import styles from './OfferActions.module.css';

const bem = cn('OfferActions');

type Props = {
  productId: string;
  availableQuantity: number;
  requestedQuantity?: number;
};

export const OfferActions = ({
  productId,
  availableQuantity,
  requestedQuantity,
}: Props): React.JSX.Element => {
  const { accept, decline, isPending } = useOfferActions(productId);

  return (
    <div className={styles[bem()]}>
      <Alert
        description={`${describeShortage(availableQuantity, requestedQuantity)} Примите доступное количество или откажитесь от заказа.`}
        title="Товаров меньше, чем вы выбрали"
        variant="error"
      />
      <div className={styles[bem('buttons')]}>
        <Button
          loading={isPending}
          onClick={() => accept({ quantity: availableQuantity })}
          size="large"
          variant="primary"
        >
          Принять
        </Button>
        <Button loading={isPending} onClick={decline} size="large" variant="danger">
          Отказаться
        </Button>
      </div>
    </div>
  );
};
