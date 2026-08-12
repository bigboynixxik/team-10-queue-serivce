import { cn } from '@shared/lib';
import { Alert, Spinner } from '@ui';

import { useSellerProducts } from '../model/useSellerProductsPage';
import { SellerProductCard } from './SellerProductCard';
import styles from './SellerProductsPage.module.css';

const bem = cn('SellerProductsPage');

export const SellerProductsPage = (): React.JSX.Element => {
  const { data: products, isPending, isError } = useSellerProducts();

  if (isPending) {
    return (
      <main className={styles[bem('placeholder')]}>
        <Spinner size="large" />
      </main>
    );
  }

  if (isError || !products) {
    return (
      <main className={styles[bem('placeholder')]}>
        <Alert title="Не удалось загрузить товары" variant="error" />
      </main>
    );
  }

  return (
    <main className={styles[bem()]}>
      <h1 className={styles[bem('title')]}>Мои товары</h1>
      <div className={styles[bem('list')]}>
        {products.map((product) => (
          <SellerProductCard key={product.id} product={product} />
        ))}
      </div>
    </main>
  );
};
