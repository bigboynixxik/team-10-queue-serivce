import type { Membership } from '@entities/queue';
import type { Nullable } from '@shared/model';
import { Alert, Descriptions, Tag } from '@ui';

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

export const QueueStatusView = ({ membership, secondsLeft }: Props) => {
  const [title, description] = content[membership.status];
  const deadline = secondsLeft === null ? null : `${secondsLeft} сек.`;

  return (
    <>
      <Alert
        type={membership.status === 'PURCHASED' ? 'success' : 'info'}
        showIcon
        message={title}
        description={description}
      />
      <Descriptions size="small" column={1} bordered>
        <Descriptions.Item label="Статус">
          <Tag color="green">{membership.status}</Tag>
        </Descriptions.Item>
        {membership.quantity && (
          <Descriptions.Item label="Количество">{membership.quantity} шт.</Descriptions.Item>
        )}
        {membership.available_quantity && (
          <Descriptions.Item label="Доступно">
            {membership.available_quantity} шт.
          </Descriptions.Item>
        )}
        {deadline && <Descriptions.Item label="Осталось">{deadline}</Descriptions.Item>}
      </Descriptions>
    </>
  );
};
