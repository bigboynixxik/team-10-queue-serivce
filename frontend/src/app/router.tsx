import { lazy } from 'react';
import { createBrowserRouter, redirect } from 'react-router-dom';

import { AppLayout } from '@app/layout/AppLayout';
import { APP_BASENAME, appPath } from '@shared/config';

const HomePage = lazy(() => import('@pages/home').then(({ HomePage }) => ({ default: HomePage })));

const PaymentSuccessPage = lazy(() =>
  import('@pages/payment-success').then(({ PaymentSuccessPage }) => ({
    default: PaymentSuccessPage,
  })),
);

const OrderInfoPage = lazy(() =>
  import('@pages/order-info').then(({ OrderInfoPage }) => ({ default: OrderInfoPage })),
);

export const router = createBrowserRouter([
  {
    path: '/',
    loader: ({ request }) => {
      const url = new URL(request.url);
      return redirect(`${APP_BASENAME}${url.search}`);
    },
  },
  {
    path: APP_BASENAME,
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
        loader: ({ params }) => redirect(appPath(`/order-info/${params.productId}`)),
      },
      {
        path: 'payment-success',
        element: <PaymentSuccessPage />,
      },
    ],
  },
]);
