import { cn } from '@shared/lib';
import { Alert, Spinner } from '@ui';

import { useSellerProductStats } from '../model/useSellerProductStats';
import styles from './SellerProductStats.module.css';

const bem = cn('SellerProductStats');

type Props = {
  productId: string;
  price: number;
};

export const SellerProductStats = ({ productId, price }: Props): React.JSX.Element => {
  const { items, isPending, isError } = useSellerProductStats(productId, price);

  if (isPending) {
    return (
      <div className={styles[bem('placeholder')]}>
        <Spinner />
      </div>
    );
  }

  if (isError || items.length === 0) {
    return <Alert title="Пока что нет статистики по этому товару" variant="error" />;
  }

  return (
    <section className={styles[bem()]} aria-label="Статистика товара">
      <div className={styles[bem('grid')]}>
        {items.map((item) => (
          <article className={styles[bem('card')]} key={item.label}>
            <p className={styles[bem('value')]}>{item.value}</p>
            <p className={styles[bem('label')]}>{item.label}</p>
          </article>
        ))}
      </div>
    </section>
  );
};
