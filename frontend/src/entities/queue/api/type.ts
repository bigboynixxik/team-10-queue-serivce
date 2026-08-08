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

export const RightValidationSchema = z.object({ valid: z.literal(true) });

export const JoinPayloadSchema = z.object({
  quantity: z.number().int().positive(),
});

export const AcceptOfferPayloadSchema = JoinPayloadSchema;

export type Membership = z.infer<typeof MembershipSchema>;
export type MembershipStatus = z.infer<typeof MembershipStatusSchema>;
export type JoinPayload = z.infer<typeof JoinPayloadSchema>;
export type AcceptOfferPayload = z.infer<typeof AcceptOfferPayloadSchema>;
