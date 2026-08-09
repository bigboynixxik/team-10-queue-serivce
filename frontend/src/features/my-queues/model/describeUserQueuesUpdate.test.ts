import { describe, expect, test } from '@rstest/core';

import type { UserQueue } from '@entities/queue';

import { describeUserQueuesUpdate } from './describeUserQueuesUpdate';

const title = (productId: string) => `Товар ${productId}`;

const queue = (
  overrides: Partial<UserQueue> & Pick<UserQueue, 'product_id' | 'status'>,
): UserQueue => overrides;

describe('describeUserQueuesUpdate', () => {
  test('skips terminal statuses on first snapshot', () => {
    const message = describeUserQueuesUpdate(
      [
        queue({ product_id: '1', status: 'PURCHASED' }),
        queue({ product_id: '2', status: 'QUEUED', position: 3 }),
      ],
      undefined,
      title,
    );

    expect(message).toBe('Товар 2 — вы в очереди, позиция 3');
  });

  test('describes status change', () => {
    const previous = [queue({ product_id: '1', status: 'QUEUED', position: 2 })];
    const current = [queue({ product_id: '1', status: 'RIGHT_ACTIVE' })];

    expect(describeUserQueuesUpdate(current, previous, title)).toBe(
      'Товар 1 — нужно оплатить товар',
    );
  });

  test('describes position-only change while queued', () => {
    const previous = [queue({ product_id: '1', status: 'QUEUED', position: 5 })];
    const current = [queue({ product_id: '1', status: 'QUEUED', position: 2 })];

    expect(describeUserQueuesUpdate(current, previous, title)).toBe('Товар 1 — позиция 2');
  });

  test('describes removed queue', () => {
    const previous = [queue({ product_id: '1', status: 'QUEUED', position: 1 })];

    expect(describeUserQueuesUpdate([], previous, title)).toBe('Товар 1 — очередь покинута');
  });

  test('returns fallback when nothing meaningful changed', () => {
    const previous = [queue({ product_id: '1', status: 'QUEUED', position: 1 })];
    const current = [queue({ product_id: '1', status: 'QUEUED', position: 1 })];

    expect(describeUserQueuesUpdate(current, previous, title)).toBe('Список очередей обновлён');
  });

  test('joins multiple changes', () => {
    const previous = [
      queue({ product_id: '1', status: 'QUEUED', position: 2 }),
      queue({ product_id: '2', status: 'QUEUED', position: 1 }),
    ];
    const current = [queue({ product_id: '1', status: 'OFFER_PENDING' })];

    expect(describeUserQueuesUpdate(current, previous, title)).toBe(
      'Товар 1 — доступно меньше товара, подтвердите количество; Товар 2 — очередь покинута',
    );
  });
});
