import { z } from 'zod';

export const SellerStatsSchema = z.object({
  total_stock: z.number().int().nonnegative(),
  total_contenders: z.number().int().nonnegative(),
  used_rights_count: z.number().int().nonnegative(),
  expired_rights_count: z.number().int().nonnegative(),
  soldout_count: z.number().int().nonnegative(),
  dropoff_count: z.number().int().nonnegative(),
  avg_payment_time: z.number().int().nonnegative().nullable(),
  avg_dropoff_time: z.number().int().nonnegative().nullable(),
});

export type SellerStats = z.infer<typeof SellerStatsSchema>;
