import { useQuery } from '@tanstack/react-query';

import { sellerStatsQueries } from '@entities/seller-stat';

import { deficitCoefficient, formatDuration, formatMoney } from '../lib/formatStats';

export const useSellerProductStats = (productId: string, price: number) => {
  const query = useQuery({
    ...sellerStatsQueries.byProductId(productId),
    enabled: Boolean(productId),
  });

  const stats = query.data;

  const items = stats
    ? [
        {
          label: 'Коэффициент дефицита',
          value: `×${deficitCoefficient(stats.total_stock, stats.total_contenders)}`,
        },
        {
          label: 'Потерянная выручка',
          value: formatMoney(stats.soldout_count * price),
        },
        { label: 'Цена', value: formatMoney(price) },
        { label: 'Sold out', value: String(stats.soldout_count) },
        {
          label: 'Право выдано, не оплачено',
          value: String(stats.expired_rights_count),
        },
        {
          label: 'Право выдано и оплачено',
          value: String(stats.used_rights_count),
        },
        {
          label: 'Время от права до оплаты',
          value: formatDuration(stats.avg_payment_time),
        },
        { label: 'Вышли из очереди', value: String(stats.dropoff_count) },
        {
          label: 'Время в очереди до выхода',
          value: formatDuration(stats.avg_dropoff_time),
        },
      ]
    : [];

  return {
    items,
    isPending: Boolean(productId) && query.isPending,
    isError: Boolean(productId) && query.isError,
  };
};
