import type { Nullable } from '@shared/model';

export type WebSocketHandler<T> = (payload: T) => void;

export type WebSocketClientOptions = {
  url: string;
  protocols?: string | string[];
};

export type WebSocketListeners<T> = {
  onMessage: WebSocketHandler<T>;
  onError?: (error: unknown) => void;
  onClose?: () => void;
};

export default class WebSocketClient<T = unknown> {
  private socket: Nullable<WebSocket> = null;

  constructor(private readonly options: WebSocketClientOptions) {}

  public connect({ onMessage, onError, onClose }: WebSocketListeners<T>): void {
    this.disconnect();

    const socket = new WebSocket(this.options.url, this.options.protocols);
    this.socket = socket;

    socket.addEventListener('message', (event) => {
      try {
        onMessage(JSON.parse(String(event.data)) as T);
      } catch (error) {
        onError?.(error);
      }
    });
    socket.addEventListener('error', (event) => onError?.(event));
    socket.addEventListener('close', () => onClose?.());
  }

  public disconnect(): void {
    this.socket?.close();
    this.socket = null;
  }
}
