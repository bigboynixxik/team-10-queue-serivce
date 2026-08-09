import { beforeEach, describe, expect, rs, test } from '@rstest/core';
import { act, renderHook } from '@testing-library/react';

import type { UserQueue } from '@entities/queue';

const info = rs.fn();
let onUpdate: ((queues: UserQueue[]) => void) | undefined;
const useUserQueuesLiveUpdates = rs.fn(
  (_userId: string, options?: { onUpdate?: typeof onUpdate }) => {
    onUpdate = options?.onUpdate;
  },
);

rs.mock('@ui', () => ({
  useToast: () => ({ info, error: rs.fn(), success: rs.fn() }),
}));

rs.mock('@entities/queue', () => ({
  useUserQueuesLiveUpdates,
  isTerminalStatus: (status?: string) =>
    status === 'DECLINED' || status === 'PURCHASED' || status === 'SOLD_OUT',
}));

rs.mock('@tanstack/react-query', () => ({
  useQuery: () => ({
    data: [{ id: 'p1', title: 'Наушники' }],
  }),
}));

rs.mock('@entities/product', () => ({
  productQueries: {
    list: () => ({ queryKey: ['products', 'list'] }),
  },
}));

const { useMyQueuesLiveUpdates } = await import('./useMyQueuesLiveUpdates');

describe('useMyQueuesLiveUpdates', () => {
  beforeEach(() => {
    info.mockReset();
    useUserQueuesLiveUpdates.mockClear();
    onUpdate = undefined;
  });

  test('wires userId into live updates', () => {
    renderHook(() => useMyQueuesLiveUpdates('user-1'));

    expect(useUserQueuesLiveUpdates).toHaveBeenCalledWith(
      'user-1',
      expect.objectContaining({ onUpdate: expect.any(Function) }),
    );
  });

  test('toasts described update and uses product title fallback', () => {
    renderHook(() => useMyQueuesLiveUpdates('user-1'));

    act(() => {
      onUpdate?.([{ product_id: 'p1', status: 'QUEUED', position: 2 }]);
    });

    expect(info).toHaveBeenCalledWith(
      {
        title: 'Обновление очередей',
        description: 'Наушники — вы в очереди, позиция 2',
      },
      { duration: 4000 },
    );

    act(() => {
      onUpdate?.([{ product_id: 'missing', status: 'RIGHT_ACTIVE' }]);
    });

    expect(info).toHaveBeenLastCalledWith(
      {
        title: 'Обновление очередей',
        description: expect.stringContaining('missing'),
      },
      { duration: 4000 },
    );
  });
});
