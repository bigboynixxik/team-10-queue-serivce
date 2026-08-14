import { beforeEach, describe, expect, rs, test } from '@rstest/core';

const join = rs.fn();
const acceptOffer = rs.fn();
const declineOffer = rs.fn();
const invalidateQueries = rs.fn();

rs.mock('../api/QueueApi', () => ({
  queueApi: { join, acceptOffer, declineOffer },
}));

rs.mock('@shared/lib', () => ({
  queryClient: { invalidateQueries },
}));

const { queueMutations } = await import('./mutations');

describe('queueMutations', () => {
  beforeEach(() => {
    join.mockReset();
    acceptOffer.mockReset();
    declineOffer.mockReset();
    invalidateQueries.mockReset();
  });

  test('join calls api and invalidates membership on success', async () => {
    join.mockResolvedValue({ status: 'QUEUED' });
    const options = queueMutations.join('p1');

    await expect(options.mutationFn?.({ quantity: 2 }, {} as never)).resolves.toEqual({
      status: 'QUEUED',
    });
    expect(join).toHaveBeenCalledWith('p1', { quantity: 2 });

    await options.onSuccess?.({ status: 'QUEUED' }, { quantity: 2 }, undefined, undefined as never);
    expect(invalidateQueries).toHaveBeenCalledWith({
      queryKey: ['queue', 'membership', 'p1'],
    });
    expect(invalidateQueries).toHaveBeenCalledWith({
      queryKey: ['queue', 'user-queues'],
    });
  });

  test('acceptOffer and declineOffer invalidate membership', async () => {
    acceptOffer.mockResolvedValue({ status: 'RIGHT_ACTIVE' });
    declineOffer.mockResolvedValue(undefined);

    const accept = queueMutations.acceptOffer('p1');
    const decline = queueMutations.declineOffer('p1');

    await accept.mutationFn?.({ quantity: 1 }, {} as never);
    await decline.mutationFn?.(undefined as never, {} as never);

    await accept.onSuccess?.(
      { status: 'RIGHT_ACTIVE' },
      { quantity: 1 },
      undefined,
      undefined as never,
    );
    await decline.onSuccess?.(undefined, undefined, undefined, undefined as never);

    expect(acceptOffer).toHaveBeenCalledWith('p1', { quantity: 1 });
    expect(declineOffer).toHaveBeenCalledWith('p1');
    expect(invalidateQueries).toHaveBeenCalledTimes(4);
  });
});
