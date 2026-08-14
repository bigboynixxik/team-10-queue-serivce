export const ERROR_TOAST_DURATION = 5000;
export const STATUS_TOAST_DURATION = 25000;

type ToastKeyContent = {
  title: string;
  description: string;
};

export const toastKey = (content: ToastKeyContent, variant: string): string =>
  `${variant}:${content.title}:${content.description}`;
