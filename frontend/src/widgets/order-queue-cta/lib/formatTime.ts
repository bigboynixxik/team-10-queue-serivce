export const formatTimeLeft = (secondsLeft: number): string => {
  const minutes = Math.floor(secondsLeft / 60);
  const seconds = secondsLeft % 60;

  return `${String(minutes).padStart(2, '0')}:${String(seconds).padStart(2, '0')}`;
};

export const formatEta = (etaSeconds: number): string => {
  if (etaSeconds < 60) return 'менее минуты';

  const minutes = Math.floor(etaSeconds / 60);
  const seconds = etaSeconds % 60;

  return seconds ? `${minutes} мин. ${seconds} сек.` : `${minutes} мин.`;
};
