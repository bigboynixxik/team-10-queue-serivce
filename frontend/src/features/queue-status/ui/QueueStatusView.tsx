import type { Membership } from '@entities/queue';
import { useToast } from '@ui';
import { useEffect, useRef } from 'react';

const content = {
  QUEUED: ['Вы в очереди', 'Ожидайте, пока товар станет доступен.'],
  OFFER_PENDING: ['Доступно меньше товара', 'Выберите количество или откажитесь от предложения.'],
  RIGHT_ACTIVE: ['Ваша очередь', 'Оплатите товар до окончания таймера.'],
  DECLINED: ['Вы исключены из очереди', 'Время на действие закончилось.'],
  PURCHASED: ['Покупка оформлена', 'Оплата подтверждена, заказ создан.'],
  SOLD_OUT: ['Товар распродан', 'К сожалению, остатки закончились.'],
} as const;

type Props = {
  membership: Membership;
  productTitle: string;
};

export const QueueStatusView = ({ membership, productTitle }: Props): null => {
  const { info, success } = useToast();
  const lastNotifiedStatus = useRef<Membership['status'] | null>(null);

  useEffect(() => {
    if (lastNotifiedStatus.current === membership.status) return;
    lastNotifiedStatus.current = membership.status;

    const [, description] = content[membership.status];
    const notify = membership.status === 'PURCHASED' ? success : info;

    notify({ title: productTitle, description });
  }, [info, membership.status, productTitle, success]);

  return null;
};
