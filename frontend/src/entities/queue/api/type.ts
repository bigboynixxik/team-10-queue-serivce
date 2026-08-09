import { z } from 'zod';

export const MembershipStatusSchema = z.enum([
  'QUEUED',
  'RIGHT_ACTIVE',
  'OFFER_PENDING',
  'DECLINED',
  'PURCHASED',
  'SOLD_OUT',
]);

export const MembershipSchema = z.object({
  status: MembershipStatusSchema,
  token: z.string().optional(),
  quantity: z.number().int().positive().optional(),
  available_quantity: z.number().int().positive().optional(),
  expires_at: z.string().datetime().optional(),
});

export const UserQueueSchema = MembershipSchema.extend({
  product_id: z.string().min(1),
  position: z.number().int().positive().optional(),
  eta_seconds: z.number().int().nonnegative().optional(),
});

export const UserQueuesSchema = z.array(UserQueueSchema);

export const RightValidationSchema = z.object({ valid: z.literal(true) });

export const QueueStatsSchema = z.object({
  waiting: z.number().int().nonnegative(),
  holding_right: z.number().int().nonnegative(),
  pending_offer: z.number().int().nonnegative(),
  available: z.number().int().nonnegative(),
  product_count: z.number().int().nonnegative(),
});

export const JoinPayloadSchema = z.object({
  quantity: z.number().int().positive(),
});

export const AcceptOfferPayloadSchema = JoinPayloadSchema;

export type Membership = z.infer<typeof MembershipSchema>;
export type QueueStats = z.infer<typeof QueueStatsSchema>;
export type MembershipStatus = z.infer<typeof MembershipStatusSchema>;
export type UserQueue = z.infer<typeof UserQueueSchema>;
export type JoinPayload = z.infer<typeof JoinPayloadSchema>;
export type AcceptOfferPayload = z.infer<typeof AcceptOfferPayloadSchema>;
