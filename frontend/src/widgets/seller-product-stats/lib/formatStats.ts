export const formatMoney = (value: number): string => `${value.toLocaleString('ru-RU')} ₽`;

export const formatDuration = (seconds: number): string => {
  const minutes = Math.floor(seconds / 60);
  const rest = seconds % 60;

  if (minutes === 0) return `${rest} с`;
  if (rest === 0) return `${minutes} мин`;

  return `${minutes} мин ${rest} с`;
};

export const formatDeficit = (stock: number, claimants: number, coefficient: number): string =>
  `на ${stock} шт. претендуют ${claimants} чел. (×${coefficient})`;
