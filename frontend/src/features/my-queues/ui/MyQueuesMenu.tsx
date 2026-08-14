import { cn } from '@shared/lib';
import { Button } from '@ui';

import { useMyQueuesMenu } from '../model/useMyQueuesMenu';
import { MyQueueItem } from './MyQueueItem';
import styles from './MyQueuesMenu.module.css';

const bem = cn('MyQueuesMenu');

export const MyQueuesMenu = (): React.JSX.Element => {
  const {
    menuRef,
    isOpen,
    toggleOpen,
    openQueue,
    activeQueues,
    productTitles,
    isPending,
    isError,
  } = useMyQueuesMenu();

  return (
    <div className={styles[bem()]} ref={menuRef}>
      <Button
        aria-expanded={isOpen}
        aria-haspopup="menu"
        aria-label="Открыть мои очереди"
        className={styles[bem('trigger')]}
        onClick={toggleOpen}
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
            {!isPending && !isError && activeQueues.length === 0 && (
              <p className={styles[bem('message')]}>Вы пока не состоите ни в одной очереди.</p>
            )}
            {activeQueues.map((queue) => (
              <MyQueueItem
                key={queue.product_id}
                onOpen={openQueue}
                queue={queue}
                title={productTitles.get(queue.product_id) ?? queue.product_id}
              />
            ))}
          </div>
        </div>
      )}
    </div>
  );
};
