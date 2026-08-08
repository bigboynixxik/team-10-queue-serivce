import { AppLayout } from '@app/layout/AppLayout';
import { APP_BASENAME } from '@shared/config';
import { lazy } from 'react';
import { createBrowserRouter } from 'react-router-dom';

const HomePage = lazy(() => import('@pages/home').then(({ HomePage }) => ({ default: HomePage })));

const QueuePage = lazy(() =>
  import('@pages/queue').then(({ QueuePage }) => ({ default: QueuePage })),
);

const PaymentSuccessPage = lazy(() =>
  import('@pages/payment-success').then(({ PaymentSuccessPage }) => ({
    default: PaymentSuccessPage,
  })),
);

const OrderInfoPage = lazy(() =>
  import('@pages/order-info').then(({ OrderInfoPage }) => ({ default: OrderInfoPage })),
);

export const router = createBrowserRouter(
  [
    {
      path: '/',
      element: <AppLayout />,
      children: [
        {
          index: true,
          element: <HomePage />,
        },
        {
          path: 'order-info/:productId',
          element: <OrderInfoPage />,
        },
        {
          path: 'queue/:productId',
          element: <QueuePage />,
        },
        {
          path: 'payment-success',
          element: <PaymentSuccessPage />,
        },
      ],
    },
  ],
  {
    basename: APP_BASENAME,
  },
);
