import { getApiErrorMessages } from '@shared/api';
import { App } from '@ui';

export type ErrorNotifier = (error: unknown, fallback?: string) => void;

/**
 * Single funnel for turning API rejections into user-facing toasts, so every
 * feature reports failures the same way.
 */
export const useErrorNotifier = (): ErrorNotifier => {
  const { message } = App.useApp();

  return (error, fallback) => {
    for (const text of getApiErrorMessages(error, fallback)) {
      message.error(text);
    }
  };
};
