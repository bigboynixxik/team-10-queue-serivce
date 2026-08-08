import { lazy } from 'react';
import { createBrowserRouter } from 'react-router-dom';

import { AppLayout } from '@app/layout/AppLayout';
import { APP_BASENAME } from '@shared/config';

const HomePage = lazy(() =>
	import('@pages/home').then(({ HomePage }) => ({ default: HomePage })),
);

const QueuePage = lazy(() =>
	import('@pages/queue').then(({ QueuePage }) => ({ default: QueuePage })),
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
          path: 'queue/:productId',
          element: <QueuePage />,
        },
      ],
    },
  ],
  {
    basename: APP_BASENAME,
  },
);
