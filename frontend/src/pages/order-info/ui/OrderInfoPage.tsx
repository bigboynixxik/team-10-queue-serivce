import { useNavigate } from 'react-router-dom';

import { appPath } from '@shared/config';
import { cn } from '@shared/lib';
import { Alert, Button, Spinner } from '@ui';
import { OrderQueueCta } from '@widgets/order-queue-cta';
import { ProductCatalog } from '@widgets/product-catalog';

import { useOrderInfoPage } from '../model/useOrderInfoPage';
import styles from './OrderInfoPage.module.css';

const bem = cn('OrderInfoPage');

export const OrderInfoPage = (): React.JSX.Element => {
  const navigate = useNavigate();
  const { product, isPending, isError } = useOrderInfoPage();

  if (isPending) {
    return (
      <main className={styles[bem('placeholder')]}>
        <Spinner size="large" />
      </main>
    );
  }

  if (isError || !product) {
    return (
      <main className={styles[bem('placeholder')]}>
        <Alert title="Товар не найден" variant="error" />
      </main>
    );
  }

  return (
    <main className={styles[bem()]}>
      <Button className={styles[bem('back')]} onClick={() => navigate(appPath())}>
        Назад
      </Button>
      <img alt={product.title} className={styles[bem('image')]} src={product.image} />
      <section className={styles[bem('summary')]}>
        <h1 className={styles[bem('title')]}>{product.title}</h1>
        <p className={styles[bem('price')]}>{product.price.toLocaleString('ru-RU')} ₽</p>
        <p className={styles[bem('description')]}>{product.description}</p>
        <OrderQueueCta product={product} />
      </section>
      <section className={styles[bem('others')]}>
        <h2 className={styles[bem('others-title')]}>Другие товары</h2>
        <ProductCatalog excludeId={product.id} />
      </section>
    </main>
  );
};
