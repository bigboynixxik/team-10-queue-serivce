import { cn } from '@shared/lib';
import { Button } from '@ui';
import { useEffect, useRef, useState } from 'react';
import { useNavigate } from 'react-router-dom';

import { mockQueues } from '../model/mockQueues';

import styles from './MyQueuesMenu.module.css';

const bem = cn('MyQueuesMenu');

export const MyQueuesMenu = (): React.JSX.Element => {
  const [isOpen, setIsOpen] = useState(false);
  const menuRef = useRef<HTMLDivElement>(null);
  const navigate = useNavigate();

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
          <p className={styles[bem('title')]}>{`Мои очереди (${mockQueues.length})`}</p>
          <div className={styles[bem('list')]}>
            {mockQueues.map((queue) => (
              <button
                className={styles[bem('queue')]}
                key={queue.id}
                onClick={() => openQueue(queue.id)}
                role="menuitem"
                type="button"
              >
                <span>{queue.title}</span>
              </button>
            ))}
          </div>
        </div>
      )}
    </div>
  );
};
