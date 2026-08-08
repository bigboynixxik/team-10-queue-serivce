import type { MembershipStatus, UserQueue } from '@entities/queue';

const statusText: Record<MembershipStatus, string> = {
  QUEUED: 'вы в очереди',
  RIGHT_ACTIVE: 'подошла ваша очередь',
  OFFER_PENDING: 'доступно меньше товара',
  DECLINED: 'вы вышли из очереди',
  PURCHASED: 'покупка оформлена',
  SOLD_OUT: 'товар распродан',
};

/**
 * Turns two consecutive SSE snapshots into one line of text. The stream sends
 * the whole list every time, so the previous snapshot is the only way to tell
 * what the user should actually be told about.
 */
export const describeUserQueuesUpdate = (
  queues: UserQueue[],
  previous: UserQueue[] | undefined,
  getProductTitle: (productId: string) => string,
): string => {
  const before = new Map((previous ?? []).map((queue) => [queue.product_id, queue]));
  const changes: string[] = [];

  for (const queue of queues) {
    const title = getProductTitle(queue.product_id);
    const prev = before.get(queue.product_id);

    before.delete(queue.product_id);

    if (!prev || prev.status !== queue.status) {
      changes.push(`${title} — ${statusText[queue.status]}`);
    } else if (queue.position && prev.position !== queue.position) {
      changes.push(`${title} — позиция ${queue.position}`);
    }
  }

  for (const queue of before.values()) {
    changes.push(`${getProductTitle(queue.product_id)} — очередь покинута`);
  }

  return changes.length > 0 ? changes.join('; ') : 'Список очередей обновлён';
};
