import { AppLayout } from '@app/layout/AppLayout';
import { APP_BASENAME } from '@shared/config';
import { lazy } from 'react';
import { createBrowserRouter, redirect } from 'react-router-dom';

const HomePage = lazy(() => import('@pages/home').then(({ HomePage }) => ({ default: HomePage })));

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
          // The queue lives on the product page now; old links must not dead-end.
          path: 'queue/:productId',
          loader: ({ params }) => redirect(`/order-info/${params.productId}`),
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
