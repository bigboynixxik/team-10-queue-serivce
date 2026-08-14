import { describe, expect, test } from '@rstest/core';

import {
  JoinPayloadSchema,
  MembershipSchema,
  MembershipStatusSchema,
  QueueStatsSchema,
  UserQueuesSchema,
} from './type';

describe('queue api schemas', () => {
  test('accepts all membership statuses', () => {
    for (const status of [
      'QUEUED',
      'RIGHT_ACTIVE',
      'OFFER_PENDING',
      'DECLINED',
      'PURCHASED',
      'SOLD_OUT',
    ]) {
      expect(MembershipStatusSchema.parse(status)).toBe(status);
    }
  });

  test('rejects unknown membership status', () => {
    expect(() => MembershipStatusSchema.parse('WAITING')).toThrow();
  });

  test('parses membership with optional fields', () => {
    expect(
      MembershipSchema.parse({
        status: 'RIGHT_ACTIVE',
        token: 'tok',
        quantity: 2,
        available_quantity: 1,
        expires_at: '2026-08-09T10:00:00.000Z',
      }),
    ).toMatchObject({ status: 'RIGHT_ACTIVE', token: 'tok', quantity: 2 });
  });

  test('requires positive join quantity', () => {
    expect(JoinPayloadSchema.parse({ quantity: 3 })).toEqual({ quantity: 3 });
    expect(() => JoinPayloadSchema.parse({ quantity: 0 })).toThrow();
    expect(() => JoinPayloadSchema.parse({ quantity: -1 })).toThrow();
  });

  test('parses user queues list used by SSE', () => {
    expect(
      UserQueuesSchema.parse([
        { product_id: 'p1', status: 'QUEUED', position: 1, eta_seconds: 30 },
      ]),
    ).toHaveLength(1);
  });

  test('parses queue stats', () => {
    expect(
      QueueStatsSchema.parse({
        waiting: 0,
        holding_right: 1,
        pending_offer: 0,
        available: 2,
        product_count: 5,
      }),
    ).toMatchObject({ product_count: 5 });
  });
});
