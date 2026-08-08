import { Alert } from '@ui';
import { QueueSession } from '@widgets/queue-session';
import { useParams } from 'react-router-dom';

export function QueuePage() {
  const { productId } = useParams();

  return (
    <main className="queue-page">
      {productId ? (
        <QueueSession productId={productId} />
      ) : (
        <Alert type="error" message="Товар не выбран" />
      )}
    </main>
  );
}
