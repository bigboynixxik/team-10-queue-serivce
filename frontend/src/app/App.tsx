import { QueryProvider } from '@app/providers/QueryProvider';
import { router } from '@app/router';
import { HelmetProvider } from 'react-helmet-async';
import { RouterProvider } from 'react-router-dom';

export const App = (): React.JSX.Element => {
  return (
    <HelmetProvider>
      <QueryProvider>
        <RouterProvider router={router} />
      </QueryProvider>
    </HelmetProvider>
  );
}
