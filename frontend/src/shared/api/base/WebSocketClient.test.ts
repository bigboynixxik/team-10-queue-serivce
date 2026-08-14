import { afterEach, beforeEach, describe, expect, rs, test } from '@rstest/core';

import WebSocketClient from './WebSocketClient';

type Handler = (event: unknown) => void;

class FakeWebSocket {
  static instances: FakeWebSocket[] = [];
  readonly url: string;
  readonly protocols?: string | string[];
  close = rs.fn();
  private readonly listeners = new Map<string, Handler[]>();

  constructor(url: string, protocols?: string | string[]) {
    this.url = url;
    this.protocols = protocols;
    FakeWebSocket.instances.push(this);
  }

  addEventListener(type: string, handler: Handler) {
    const list = this.listeners.get(type) ?? [];
    list.push(handler);
    this.listeners.set(type, list);
  }

  emit(type: string, event: unknown) {
    for (const handler of this.listeners.get(type) ?? []) {
      handler(event);
    }
  }
}

describe('WebSocketClient', () => {
  beforeEach(() => {
    FakeWebSocket.instances = [];
    rs.stubGlobal('WebSocket', FakeWebSocket);
  });

  afterEach(() => {
    rs.unstubAllGlobals();
  });

  test('connects and parses json messages', () => {
    const onMessage = rs.fn();
    const client = new WebSocketClient({ url: 'ws://example/socket', protocols: 'proto' });

    client.connect({ onMessage });

    const socket = FakeWebSocket.instances[0];
    expect(socket.url).toBe('ws://example/socket');
    expect(socket.protocols).toBe('proto');

    socket.emit('message', { data: JSON.stringify({ ok: true }) });
    expect(onMessage).toHaveBeenCalledWith({ ok: true });
  });

  test('forwards parse errors, socket errors and close', () => {
    const onMessage = rs.fn();
    const onError = rs.fn();
    const onClose = rs.fn();
    const client = new WebSocketClient({ url: 'ws://example/socket' });

    client.connect({ onMessage, onError, onClose });
    const socket = FakeWebSocket.instances[0];

    socket.emit('message', { data: '{bad' });
    expect(onError).toHaveBeenCalled();
    expect(onMessage).not.toHaveBeenCalled();

    const errorEvent = { type: 'error' };
    socket.emit('error', errorEvent);
    expect(onError).toHaveBeenCalledWith(errorEvent);

    socket.emit('close', {});
    expect(onClose).toHaveBeenCalled();
  });

  test('disconnect closes previous socket on reconnect', () => {
    const client = new WebSocketClient({ url: 'ws://example/socket' });

    client.connect({ onMessage: rs.fn() });
    const first = FakeWebSocket.instances[0];

    client.connect({ onMessage: rs.fn() });

    expect(first.close).toHaveBeenCalled();
    expect(FakeWebSocket.instances).toHaveLength(2);

    client.disconnect();
    expect(FakeWebSocket.instances[1].close).toHaveBeenCalled();
  });
});
