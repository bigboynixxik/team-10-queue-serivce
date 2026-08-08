import ruRU from 'antd/locale/ru_RU';
import type { PropsWithChildren } from 'react';

import { App as AntdApp, ConfigProvider } from '@ui';

const theme = {
  token: {
    colorPrimary: '#00aa5b',
    fontFamily: 'Manrope, sans-serif',
  },
};

/**
 * `AntdApp` is what gives hooks such as `App.useApp()` a context to hang the
 * message/notification portals on.
 */
export const AntdProvider = ({ children }: PropsWithChildren): React.JSX.Element => {
  return (
    <ConfigProvider locale={ruRU} theme={theme}>
      <AntdApp>{children}</AntdApp>
    </ConfigProvider>
  );
};
