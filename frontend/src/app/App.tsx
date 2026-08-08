import { RouterProvider } from 'react-router-dom';

import { AntdProvider } from '@app/providers/AntdProvider';
import { QueryProvider } from '@app/providers/QueryProvider';
import { router } from '@app/router';

export const App = (): React.JSX.Element => {
  return (
    <QueryProvider>
      <AntdProvider>
        <RouterProvider router={router} />
      </AntdProvider>
    </QueryProvider>
  );
};
