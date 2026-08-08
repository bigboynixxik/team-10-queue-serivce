import { cn } from '@shared/lib';
import { Alert } from '@ui';
import { QueueSession } from '@widgets/queue-session';

import { useQueuePage } from '../model/useQueuePage';
import styles from './QueuePage.module.css';

const bem = cn('QueuePage');

export const QueuePage = (): React.JSX.Element => {
  const { productId } = useQueuePage();

  return (
    <main className={styles[bem()]}>
      {productId ? (
        <QueueSession productId={productId} />
      ) : (
        <Alert title="Товар не выбран" variant="error" />
      )}
    </main>
  );
};
