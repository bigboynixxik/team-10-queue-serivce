import { beforeEach, describe, expect, rs, test } from '@rstest/core';

import { HttpError } from '@shared/api';

const getMe = rs.fn();
const getStats = rs.fn();
const getAll = rs.fn();

rs.mock('../api/QueueApi', () => ({
  queueApi: { getMe, getStats },
}));

rs.mock('../api/UserQueuesApi', () => ({
  userQueuesApi: { getAll },
}));

const { queueMembershipQueryKey, queueQueries, queueStatsQueryKey, userQueuesQueryKey } =
  await import('./queries');

describe('queue query keys', () => {
  test('builds stable keys', () => {
    expect(queueMembershipQueryKey('p1')).toEqual(['queue', 'membership', 'p1']);
    expect(queueStatsQueryKey('p1')).toEqual(['queue', 'stats', 'p1']);
    expect(userQueuesQueryKey('u1')).toEqual(['queue', 'user-queues', 'u1']);
  });
});

describe('queueQueries', () => {
  beforeEach(() => {
    getMe.mockReset();
    getStats.mockReset();
    getAll.mockReset();
  });

  test('me disables retries and calls getMe', async () => {
    getMe.mockResolvedValue({ status: 'QUEUED' });
    const options = queueQueries.me('p1');

    expect(options.queryKey).toEqual(['queue', 'membership', 'p1']);
    expect(options.retry).toBe(false);
    await expect(options.queryFn?.({} as never)).resolves.toEqual({ status: 'QUEUED' });
    expect(getMe).toHaveBeenCalledWith('p1');
  });

  test('me maps 404 to null', async () => {
    getMe.mockRejectedValue(new HttpError(404, 'not in queue'));
    await expect(queueQueries.me('p1').queryFn?.({} as never)).resolves.toBeNull();
  });

  test('stats enabled only with productId', async () => {
    getStats.mockResolvedValue({ product_count: 1 });
    expect(queueQueries.stats('').enabled).toBe(false);
    expect(queueQueries.stats('p1').enabled).toBe(true);
    await queueQueries.stats('p1').queryFn?.({} as never);
    expect(getStats).toHaveBeenCalledWith('p1');
  });

  test('allForUser enabled only with userId', async () => {
    getAll.mockResolvedValue([]);
    expect(queueQueries.allForUser('').enabled).toBe(false);
    expect(queueQueries.allForUser('u1').enabled).toBe(true);
    await queueQueries.allForUser('u1').queryFn?.({} as never);
    expect(getAll).toHaveBeenCalled();
  });
});
