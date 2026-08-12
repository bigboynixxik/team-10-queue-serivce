import { useQuery } from '@tanstack/react-query';

import { sellerStatsQueries } from '@entities/seller-stat';

import { formatDeficit, formatDuration, formatMoney } from '../lib/formatStats';

export const useSellerProductStats = (productId: string) => {
  const query = useQuery({
    ...sellerStatsQueries.byProductId(productId),
    enabled: Boolean(productId),
  });

  const stats = query.data;

  const items = stats
    ? [
        {
          label: 'Коэффициент дефицита',
          value: formatDeficit(stats.stock, stats.claimants, stats.deficit_coefficient),
        },
        { label: 'Потерянная выручка', value: formatMoney(stats.lost_revenue) },
        { label: 'Цена', value: formatMoney(stats.price) },
        { label: 'Sold out', value: String(stats.sold_out_count) },
        {
          label: 'Право выдано, не оплачено',
          value: String(stats.rights_issued_unpaid),
        },
        {
          label: 'Право выдано и оплачено',
          value: String(stats.rights_issued_paid),
        },
        {
          label: 'Время от права до оплаты',
          value: formatDuration(stats.avg_right_to_payment_seconds),
        },
        { label: 'Вышли из очереди', value: String(stats.left_queue_count) },
        {
          label: 'Время в очереди до выхода',
          value: formatDuration(stats.avg_queue_time_before_leave_seconds),
        },
      ]
    : [];

  return {
    items,
    isPending: Boolean(productId) && query.isPending,
    isError: Boolean(productId) && query.isError,
  };
};
