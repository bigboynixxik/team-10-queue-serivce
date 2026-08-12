import { afterEach, beforeEach, describe, expect, rs, test } from '@rstest/core';
import { act } from '@testing-library/react';

import { createTestQueryClient, renderHookWithProviders } from '@test/render';

class FakeWebSocket {
  static instances: FakeWebSocket[] = [];
  readonly url: string;
  private readonly listeners = new Map<string, Array<(event: unknown) => void>>();
  close = rs.fn();

  constructor(url: string | URL) {
    this.url = String(url);
    FakeWebSocket.instances.push(this);
  }

  addEventListener(type: string, listener: (event: unknown) => void) {
    const list = this.listeners.get(type) ?? [];
    list.push(listener);
    this.listeners.set(type, list);
  }

  emit(type: string, data?: unknown) {
    const event =
      type === 'message' ? new MessageEvent(type, { data: JSON.stringify(data) }) : new Event(type);
    for (const listener of this.listeners.get(type) ?? []) listener(event);
  }
}

rs.mock('@shared/config', () => ({
  API_BASE_URL: 'https://api.example.com/api/v1',
}));

const { useUserQueuesLiveUpdates } = await import('./useUserQueuesLiveUpdates');
const { queueMembershipQueryKey, userQueuesQueryKey } = await import('./queries');

describe('useUserQueuesLiveUpdates', () => {
  beforeEach(() => {
    FakeWebSocket.instances = [];
    rs.stubGlobal('WebSocket', FakeWebSocket);
    rs.useFakeTimers();
  });

  afterEach(() => {
    rs.unstubAllGlobals();
    rs.useRealTimers();
  });

  test('does not open WebSocket without userId', () => {
    renderHookWithProviders(() => useUserQueuesLiveUpdates(''));

    expect(FakeWebSocket.instances).toHaveLength(0);
  });

  test('builds ws url and syncs memberships on update', () => {
    const queryClient = createTestQueryClient();
    const onUpdate = rs.fn();

    renderHookWithProviders(() => useUserQueuesLiveUpdates('user-1', { onUpdate }), { queryClient });

    const socket = FakeWebSocket.instances[0];
    expect(socket.url).toBe('wss://api.example.com/api/v1/me/queues?user_id=user-1');

    const queues = [{ product_id: 'p1', status: 'QUEUED', position: 2, token: 'tok' }];
    act(() => socket.emit('message', queues));

    expect(queryClient.getQueryData(userQueuesQueryKey('user-1'))).toEqual(queues);
    expect(queryClient.getQueryData(queueMembershipQueryKey('p1'))).toEqual({
      status: 'QUEUED',
      token: 'tok',
      quantity: undefined,
      available_quantity: undefined,
      expires_at: undefined,
    });
    expect(onUpdate).toHaveBeenCalledWith(queues);
  });

  test('invalidates malformed messages and reconnects after close', async () => {
    const queryClient = createTestQueryClient();
    const invalidateSpy = rs.spyOn(queryClient, 'invalidateQueries');
    renderHookWithProviders(() => useUserQueuesLiveUpdates('user-1'), { queryClient });

    const socket = FakeWebSocket.instances[0];
    act(() => socket.emit('message', { invalid: true }));
    expect(invalidateSpy).toHaveBeenCalledWith({ queryKey: userQueuesQueryKey('user-1') });

    act(() => socket.emit('close'));
    await act(async () => rs.advanceTimersByTime(1000));
    expect(FakeWebSocket.instances).toHaveLength(2);
  });

  test('closes socket on unmount', () => {
    const { unmount } = renderHookWithProviders(() => useUserQueuesLiveUpdates('user-1'));
    const socket = FakeWebSocket.instances[0];

    unmount();

    expect(socket.close).toHaveBeenCalled();
  });
});
