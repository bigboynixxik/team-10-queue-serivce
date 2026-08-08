import type { Membership } from '@entities/queue';
import { useToast } from '@ui';
import { useEffect, useRef } from 'react';

import styles from './QueueStatusView.module.css';

const content = {
  OFFER_PENDING: ['Доступно меньше товара', 'Выберите количество или откажитесь от предложения.'],
  RIGHT_ACTIVE: ['Ваша очередь', 'Оплатите товар до окончания таймера.'],
  DECLINED: ['Вы вышли из очереди', 'Участие в покупке завершено.'],
  PURCHASED: ['Покупка оформлена', 'Оплата подтверждена, заказ создан.'],
  SOLD_OUT: ['Товар распродан', 'К сожалению, остатки закончились.'],
} as const;

type Props = {
  membership: Membership;
  productTitle: string;
};

export const QueueStatusView = ({ membership, productTitle }: Props): React.JSX.Element | null => {
  const { info, success } = useToast();
  const lastNotifiedStatus = useRef<Membership['status'] | null>(null);

  useEffect(() => {
    if (membership.status === 'QUEUED') return;
    if (lastNotifiedStatus.current === membership.status) return;
    lastNotifiedStatus.current = membership.status;

    const [, description] = content[membership.status];
    const notify = membership.status === 'PURCHASED' ? success : info;

    notify({ title: productTitle, description });
  }, [info, membership.status, productTitle, success]);

  if (membership.status === 'QUEUED') {
    return <p className={styles.QueueStatusView}>Ожидайте...</p>;
  }

  return null;
};
