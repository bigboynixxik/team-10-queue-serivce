import { cn } from '@shared/lib';
import { Alert, DescriptionList, Spinner } from '@ui';

import { useSellerProductStats } from '../model/useSellerProductStats';
import styles from './SellerProductStats.module.css';

const bem = cn('SellerProductStats');

type Props = {
  productId: string;
};

export const SellerProductStats = ({ productId }: Props): React.JSX.Element => {
  const { items, isPending, isError } = useSellerProductStats(productId);

  if (isPending) {
    return (
      <div className={styles[bem('placeholder')]}>
        <Spinner />
      </div>
    );
  }

  if (isError || items.length === 0) {
    return <Alert title="Не удалось загрузить статистику" variant="error" />;
  }

  return (
    <section className={styles[bem()]}>
      <h2 className={styles[bem('title')]}>Статистика товара</h2>
      <DescriptionList items={items} />
    </section>
  );
};
