import { StrictMode} from "react";
import { HelmetProvider } from 'react-helmet-async';
import { RouterProvider } from 'react-router-dom';

import { QueryProvider } from '@app/providers/QueryProvider';
import { router } from '@app/router';

export const App = (): React.JSX.Element => {
  return (
    <StrictMode>
      <HelmetProvider>
        <QueryProvider>
          <RouterProvider router={router} />
        </QueryProvider>
      </HelmetProvider>
    </StrictMode>
  );
}
