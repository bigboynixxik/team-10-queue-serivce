export const describeShortage = (availableQuantity: number, requestedQuantity?: number): string =>
  requestedQuantity === undefined
    ? `Осталось только ${availableQuantity} шт.`
    : `Вы выбрали ${requestedQuantity} шт., а осталось только ${availableQuantity} шт.`;
