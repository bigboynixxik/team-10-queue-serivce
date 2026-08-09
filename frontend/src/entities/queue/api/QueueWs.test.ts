import { beforeEach, describe, expect, rs, test } from '@rstest/core';

const connect = rs.fn();
const disconnect = rs.fn();
let clientOptions: { url: string } | undefined;

rs.mock('@shared/api', () => ({
  getWsUrl: (productId: string, userId: string) => `ws://test/${productId}/${userId}`,
  WebSocketClient: class {
    constructor(options: { url: string }) {
      clientOptions = options;
    }
    connect = connect;
    disconnect = disconnect;
  },
}));

const { QueueWs } = await import('./QueueWs');

describe('QueueWs', () => {
  beforeEach(() => {
    connect.mockReset();
    disconnect.mockReset();
    clientOptions = undefined;
  });

  test('creates client with membership ws url', () => {
    new QueueWs('p1', 'u1');

    expect(clientOptions).toEqual({ url: 'ws://test/p1/u1' });
  });

  test('forwards parsed membership and disconnects', () => {
    const onMembership = rs.fn();
    const onError = rs.fn();
    const onClose = rs.fn();
    const socket = new QueueWs('p1', 'u1');

    socket.connect({ onMembership, onError, onClose });

    const listeners = connect.mock.calls[0][0] as {
      onMessage: (payload: unknown) => void;
      onError?: (error: unknown) => void;
      onClose?: () => void;
    };

    listeners.onMessage({ status: 'QUEUED' });
    expect(onMembership).toHaveBeenCalledWith({ status: 'QUEUED' });

    listeners.onMessage({ status: 'WAITING' });
    expect(onError).toHaveBeenCalled();

    listeners.onClose?.();
    expect(onClose).toHaveBeenCalled();

    socket.disconnect();
    expect(disconnect).toHaveBeenCalled();
  });
});
