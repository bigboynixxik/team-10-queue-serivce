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

/**
 * Turns two consecutive SSE snapshots into one line of text. The stream sends
 * the whole list every time, so the previous snapshot is the only way to tell
 * what the user should actually be told about.
 *
 * Without a previous snapshot the list is the user's history rather than news,
 * so finished queues stay silent — announcing a long sold out product as if it
 * had just happened is what the user reads as a wrong message.
 */
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

    if (!previous && isTerminalStatus(queue.status)) continue;

    if (!prev || prev.status !== queue.status) {
      changes.push(`${getProductTitle(queue.product_id)} — ${describeQueue(queue)}`);
    } else if (queue.position && prev.position !== queue.position) {
      changes.push(`${getProductTitle(queue.product_id)} — позиция ${queue.position}`);
    }
  }

  for (const queue of before.values()) {
    changes.push(`${getProductTitle(queue.product_id)} — очередь покинута`);
  }

  return changes.length > 0 ? changes.join('; ') : 'Список очередей обновлён';
};
