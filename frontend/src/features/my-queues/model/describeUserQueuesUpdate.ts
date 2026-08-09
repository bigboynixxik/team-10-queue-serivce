import { isTerminalStatus, type MembershipStatus, type UserQueue } from '@entities/queue';

const statusText: Record<MembershipStatus, string> = {
  QUEUED: 'вы в очереди',
  RIGHT_ACTIVE: 'нужно оплатить товар',
  OFFER_PENDING: 'доступно меньше товара, подтвердите количество',
  DECLINED: 'вы вышли из очереди',
  PURCHASED: 'покупка оформлена',
  SOLD_OUT: 'товар распродан',
};

const describeQueue = (queue: UserQueue): string =>
  queue.status === 'QUEUED' && queue.position
    ? `${statusText.QUEUED}, позиция ${queue.position}`
    : statusText[queue.status];

export const describeUserQueuesUpdate = (
  queues: UserQueue[],
  previous: UserQueue[] | undefined,
  getProductTitle: (productId: string) => string,
): string => {
  const before = new Map((previous ?? []).map((queue) => [queue.product_id, queue]));
  const changes: string[] = [];

  for (const queue of queues) {
    const prev = before.get(queue.product_id);

    before.delete(queue.product_id);

    // первый снимок не шумит про уже завершённые
    if (!previous && isTerminalStatus(queue.status)) continue;

    if (!prev || prev.status !== queue.status) {
      changes.push(`${getProductTitle(queue.product_id)} — ${describeQueue(queue)}`);
    } else if (queue.position && prev.position !== queue.position) {
      changes.push(`${getProductTitle(queue.product_id)} — позиция ${queue.position}`);
    }
  }

  // исчезли из списка значит вышли из очереди
  for (const queue of before.values()) {
    changes.push(`${getProductTitle(queue.product_id)} — очередь покинута`);
  }

  return changes.length > 0 ? changes.join('; ') : 'Список очередей обновлён';
};
