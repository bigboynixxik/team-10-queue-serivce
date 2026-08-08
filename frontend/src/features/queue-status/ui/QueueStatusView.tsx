import type { Membership } from '@entities/queue';
import type { Nullable } from '@shared/model';
import { Alert, DescriptionList, Tag } from '@ui';

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
  secondsLeft: Nullable<number>;
};

export const QueueStatusView = ({ membership, secondsLeft }: Props): React.JSX.Element => {
  const [title, description] = content[membership.status];
  const deadline = secondsLeft === null ? null : `${secondsLeft} сек.`;
  const items = [
    { label: 'Статус', value: <Tag variant="success">{membership.status}</Tag> },
    ...(membership.quantity ? [{ label: 'Количество', value: `${membership.quantity} шт.` }] : []),
    ...(membership.available_quantity
      ? [{ label: 'Доступно', value: `${membership.available_quantity} шт.` }]
      : []),
    ...(deadline ? [{ label: 'Осталось', value: deadline }] : []),
  ];

  return (
    <>
      <Alert
        description={description}
        title={title}
        variant={membership.status === 'PURCHASED' ? 'success' : 'info'}
      />
      <DescriptionList items={items} />
    </>
  );
};
