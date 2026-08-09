import { useQuery } from '@tanstack/react-query';
import { useParams } from 'react-router-dom';

import { productQueries } from '@entities/product';

export const useOrderInfoPage = () => {
  const { productId = '' } = useParams();
  const {
    data: product,
    isPending,
    isError,
  } = useQuery({ ...productQueries.byId(productId), enabled: Boolean(productId) });

  return {
    product,
    isPending: Boolean(productId) && isPending,
    isError: !productId || isError,
  };
};
