import { AppLayout } from '@app/layout/AppLayout';
import { HomePage } from '@pages/home';
import { QueuePage } from '@pages/queue';
import { createBrowserRouter } from 'react-router-dom';

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
          path: 'queue',
          element: <QueuePage />,
        },
      ],
    },
  ],
  {
    basename: '/avito',
  },
);
