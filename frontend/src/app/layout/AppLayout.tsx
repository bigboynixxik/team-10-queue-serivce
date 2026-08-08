import { Outlet } from 'react-router-dom';

import { useUserStore } from '@entities/user';
import { Layout } from '@ui';

export const AppLayout = (): React.JSX.Element => {
  const userId = useUserStore.use.userId();

  return (
    <Layout className="app-shell" data-user-id={userId}>
      <Outlet />
    </Layout>
  );
};
