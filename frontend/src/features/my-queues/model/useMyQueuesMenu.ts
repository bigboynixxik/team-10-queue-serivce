import { useQuery } from '@tanstack/react-query';
import { useCallback, useRef, useState } from 'react';
import { useNavigate } from 'react-router-dom';

import { productQueries } from '@entities/product';
import { isTerminalStatus, queueQueries } from '@entities/queue';
import { useUserStore } from '@entities/user';
import { appPath } from '@shared/config';

import { useMenuDismiss } from '../lib/useMenuDismiss';

export const useMyQueuesMenu = () => {
  const [isOpen, setIsOpen] = useState(false);
  const menuRef = useRef<HTMLDivElement>(null);
  const navigate = useNavigate();
  const userId = useUserStore.use.userId();
  const { data: queues = [], isError, isPending } = useQuery(queueQueries.allForUser(userId));
  const { data: products = [] } = useQuery(productQueries.list());

  const dismiss = useCallback(() => setIsOpen(false), []);

  useMenuDismiss({ menuRef, isOpen, onDismiss: dismiss });

  // A terminal membership is over for this user, so it is no longer a queue to
  // return to — the queue page would only greet them with a farewell message.
  const activeQueues = queues.filter((queue) => !isTerminalStatus(queue.status));
  const productTitles = new Map(products.map((product) => [product.id, product.title]));

  const openQueue = (productId: string) => {
    setIsOpen(false);
    navigate(appPath(`/order-info/${productId}`));
  };

  const toggleOpen = () => setIsOpen((open) => !open);

  return {
    menuRef,
    isOpen,
    toggleOpen,
    openQueue,
    activeQueues,
    productTitles,
    isPending,
    isError,
  };
};
