import { productQueries } from '@entities/product';
import { isTerminalStatus, queueQueries } from '@entities/queue';
import { useUserStore } from '@entities/user';
import { cn } from '@shared/lib';
import { useQuery } from '@tanstack/react-query';
import { Button } from '@ui';
import { useEffect, useRef, useState } from 'react';
import { useNavigate } from 'react-router-dom';

import styles from './MyQueuesMenu.module.css';

const bem = cn('MyQueuesMenu');

export const MyQueuesMenu = (): React.JSX.Element => {
  const [isOpen, setIsOpen] = useState(false);
  const menuRef = useRef<HTMLDivElement>(null);
  const navigate = useNavigate();
  const userId = useUserStore.use.userId();
  const { data: queues = [], isError, isPending } = useQuery(queueQueries.allForUser(userId));
  const { data: products = [] } = useQuery(productQueries.list());

  // A terminal membership is over for this user, so it is no longer a queue to
  // return to — the queue page would only greet them with a farewell message.
  const activeQueues = queues.filter((queue) => !isTerminalStatus(queue.status));
  const productTitles = new Map(products.map((product) => [product.id, product.title]));

  useEffect(() => {
    const closeOnOutsideClick = (event: MouseEvent) => {
      if (!menuRef.current?.contains(event.target as Node)) {
        setIsOpen(false);
      }
    };

    const closeOnEscape = (event: KeyboardEvent) => {
      if (event.key === 'Escape') {
        setIsOpen(false);
      }
    };

    document.addEventListener('mousedown', closeOnOutsideClick);
    document.addEventListener('keydown', closeOnEscape);

    return () => {
      document.removeEventListener('mousedown', closeOnOutsideClick);
      document.removeEventListener('keydown', closeOnEscape);
    };
  }, []);

  const openQueue = (productId: string) => {
    setIsOpen(false);
    navigate(`/queue/${productId}`);
  };

  return (
    <div className={styles[bem()]} ref={menuRef}>
      <Button
        aria-label="Открыть мои очереди"
        aria-expanded={isOpen}
        aria-haspopup="menu"
        className={styles[bem('trigger')]}
        onClick={() => setIsOpen((open) => !open)}
      >
        <span aria-hidden className={styles[bem('icon')]}>
          <span />
          <span />
          <span />
        </span>
      </Button>

      {isOpen && (
        <div aria-label="Мои очереди" className={styles[bem('dropdown')]} role="menu">
          <p className={styles[bem('title')]}>{`Мои очереди (${activeQueues.length})`}</p>
          <div className={styles[bem('list')]}>
            {isPending && <p className={styles[bem('message')]}>Загружаем очереди…</p>}
            {/* {isError && <p className={styles[bem('message')]}>Не удалось загрузить очереди.</p>} */}
            {!isPending && !isError && activeQueues.length === 0 && (
              <p className={styles[bem('message')]}>Вы пока не состоите ни в одной очереди.</p>
            )}
            {activeQueues.map((queue) => (
              <button
                className={styles[bem('queue')]}
                key={queue.product_id}
                onClick={() => openQueue(queue.product_id)}
                role="menuitem"
                type="button"
              >
                <span className={styles[bem('queue-title')]}>
                  {productTitles.get(queue.product_id) ?? queue.product_id}
                </span>
                {queue.status === 'QUEUED' && queue.position !== undefined && (
                  <span className={styles[bem('queue-position')]}>
                    Место в очереди: {queue.position}
                  </span>
                )}
              </button>
            ))}
          </div>
        </div>
      )}
    </div>
  );
};
