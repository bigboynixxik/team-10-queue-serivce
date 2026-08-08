import { QueryProvider } from '@app/providers/QueryProvider';
import { router } from '@app/router';
import { ToastProvider } from '@ui';
import { RouterProvider } from 'react-router-dom';

export const App = (): React.JSX.Element => {
  return (
    <QueryProvider>
      <ToastProvider>
        <RouterProvider router={router} />
      </ToastProvider>
    </QueryProvider>
  );
};
