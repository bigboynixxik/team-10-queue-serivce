import { useParams } from 'react-router-dom';

export const useQueuePage = () => {
  const { productId } = useParams();

  return { productId };
};
