export type MockQueue = {
  id: string;
  title: string;
  // position: number;
};

export const mockQueues: readonly MockQueue[] = [
  {
    id: 'wireless-headphones',
    title: 'Наушники Sennheiser Momentum 4 Wireless',
  },
  {
    id: 'gaming-console',
    title: 'Игровая консоль PlayStation 5 Slim Digital Edition',
  },
  {
    id: 'smart-watch',
    title: 'Смарт-часы HUAWEI WATCH GT 6 Pro 46 mm',
  },
];
