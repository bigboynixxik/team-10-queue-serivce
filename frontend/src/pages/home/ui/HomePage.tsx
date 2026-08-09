import { cn } from '@shared/lib';
import { ProductCatalog } from '@widgets/product-catalog';

import styles from './HomePage.module.css';

const bem = cn('HomePage');

export const HomePage = (): React.JSX.Element => {
  return (
    <main className={styles[bem()]}>
      <ProductCatalog />
    </main>
  );
};
