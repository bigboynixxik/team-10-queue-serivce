import { Link } from 'react-router-dom';

import { appPath } from '@shared/config';
import { cn } from '@shared/lib';

import styles from './PaymentSuccessPage.module.css';

const bem = cn('PaymentSuccessPage');

export const PaymentSuccessPage = (): React.JSX.Element => (
  <main className={styles[bem()]}>
    <section className={styles[bem('card')]} aria-labelledby="payment-success-title">
      <span aria-hidden className={styles[bem('icon')]}>
        ✓
      </span>
      <h1 className={styles[bem('title')]} id="payment-success-title">
        Оплата прошла успешно
      </h1>
      <p className={styles[bem('description')]}>Заказ оформлен. Спасибо за покупку!</p>
      <Link className={styles[bem('link')]} to={appPath()}>
        Ко всем товарам
      </Link>
    </section>
  </main>
);
