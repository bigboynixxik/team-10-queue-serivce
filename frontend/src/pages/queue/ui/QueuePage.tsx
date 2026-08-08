import { cn } from '@shared/lib';
import { Alert, Button } from '@ui';
import { QueueSession } from '@widgets/queue-session';
import { useNavigate } from 'react-router-dom';

import { useQueuePage } from '../model/useQueuePage';
import styles from './QueuePage.module.css';

const bem = cn('QueuePage');

export const QueuePage = (): React.JSX.Element => {
  const { productId } = useQueuePage();
  const navigate = useNavigate();

  return (
    <main className={styles[bem()]}>
      {productId && (
        <Button
          className={styles[bem('back')]}
          onClick={() => navigate(`/order-info/${productId}`)}
        >
          Назад
        </Button>
      )}
      {productId ? (
        <QueueSession productId={productId} />
      ) : (
        <Alert title="Товар не выбран" variant="error" />
      )}
    </main>
  );
};
