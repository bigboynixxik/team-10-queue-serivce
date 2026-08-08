import { getWsUrl, WebSocketClient } from '@shared/api';

import { type Membership, MembershipSchema } from './type';

export type QueueWsListeners = {
  onMembership: (membership: Membership) => void;
  onError?: (error: unknown) => void;
};

export class QueueWs {
  private readonly client: WebSocketClient<unknown>;

  constructor(productId: string, userId: string) {
    this.client = new WebSocketClient({ url: getWsUrl(productId, userId) });
  }

  public connect({ onMembership, onError }: QueueWsListeners): void {
    this.client.connect({
      onMessage: (payload) => onMembership(MembershipSchema.parse(payload)),
      onError,
    });
  }

  public disconnect(): void {
    this.client.disconnect();
  }
}
