import { getWsUrl, WebSocketClient } from '@shared/api';

import { type Membership, MembershipSchema } from './type';

export type QueueWsListeners = {
  onMembership: (membership: Membership) => void;
  onError?: (error: unknown) => void;
  onClose?: () => void;
};

export class QueueWs {
  private readonly client: WebSocketClient<unknown>;

  constructor(productId: string, userId: string) {
    this.client = new WebSocketClient({ url: getWsUrl(productId, userId) });
  }

  public connect({ onMembership, onError, onClose }: QueueWsListeners): void {
    this.client.connect({
      onMessage: (payload) => {
        const result = MembershipSchema.safeParse(payload);

        if (!result.success) {
          onError?.(result.error);
          return;
        }

        onMembership(result.data);
      },
      onError,
      onClose,
    });
  }

  public disconnect(): void {
    this.client.disconnect();
  }
}
