import { z } from 'zod';

export const SellerStatsSchema = z.object({
  product_id: z.string().min(1),
  price: z.number().int().nonnegative(),
  sold_out_count: z.number().int().nonnegative(),
  lost_revenue: z.number().int().nonnegative(),
  stock: z.number().int().positive(),
  claimants: z.number().int().nonnegative(),
  deficit_coefficient: z.number().nonnegative(),
  rights_issued_unpaid: z.number().int().nonnegative(),
  rights_issued_paid: z.number().int().nonnegative(),
  avg_right_to_payment_seconds: z.number().int().nonnegative(),
  left_queue_count: z.number().int().nonnegative(),
  avg_queue_time_before_leave_seconds: z.number().int().nonnegative(),
});

export const SellerStatsListSchema = z.array(SellerStatsSchema);

export type SellerStats = z.infer<typeof SellerStatsSchema>;
