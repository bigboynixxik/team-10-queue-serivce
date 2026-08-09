export const isPaymentSucceeded = (data: unknown): boolean => {
  if (typeof data !== 'object' || data === null) return false;

  const message = data as { source?: unknown; event?: unknown };

  return message.source === 'avito-checkout' && message.event === 'payment_succeeded';
};
