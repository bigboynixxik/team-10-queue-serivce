import { RouterProvider } from 'react-router-dom';

import { QueryProvider } from '@app/providers/QueryProvider';
import { router } from '@app/router';
import { ToastProvider } from '@ui';

export const App = (): React.JSX.Element => (
  <QueryProvider>
    <ToastProvider>
      <RouterProvider router={router} />
    </ToastProvider>
  </QueryProvider>
);
