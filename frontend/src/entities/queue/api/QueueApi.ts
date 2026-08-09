import { HttpClient } from '@shared/api';

import {
  type AcceptOfferPayload,
  AcceptOfferPayloadSchema,
  type JoinPayload,
  JoinPayloadSchema,
  type Membership,
  MembershipSchema,
  type QueueStats,
  QueueStatsSchema,
} from './type';

class QueueApi extends HttpClient {
  constructor() {
    super('queue');
  }

  public async join(productId: string, payload: JoinPayload): Promise<Membership> {
    return MembershipSchema.parse(
      await this.post<JoinPayload, unknown>({
        uri: `/${productId}/members`,
        data: JoinPayloadSchema.parse(payload),
      }),
    );
  }

  public async getMe(productId: string): Promise<Membership> {
    return MembershipSchema.parse(await this.get<unknown>({ uri: `/${productId}/members/me` }));
  }

  public async getStats(productId: string): Promise<QueueStats> {
    return QueueStatsSchema.parse(await this.get<unknown>({ uri: `/${productId}/stats` }));
  }

  public async acceptOffer(productId: string, payload: AcceptOfferPayload): Promise<Membership> {
    return MembershipSchema.parse(
      await this.patch<AcceptOfferPayload, unknown>({
        uri: `/${productId}/members/me`,
        data: AcceptOfferPayloadSchema.parse(payload),
      }),
    );
  }

  public async declineOffer(productId: string): Promise<void> {
    await this.delete({ uri: `/${productId}/members/me` });
  }
}

export const queueApi = new QueueApi();
