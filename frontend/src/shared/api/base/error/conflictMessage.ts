const QUEUE_LIMIT_REACHED = 'queue_limit_reached';

type ConflictBody = {
  status?: unknown;
  error?: unknown;
  limit?: unknown;
};

const readConflictBody = (data: unknown): ConflictBody | null => {
  if (typeof data !== 'object' || data === null) return null;
  return data as ConflictBody;
};

export const conflictMessage = (data: unknown, fallback: string | string[]): string | string[] => {
  const body = readConflictBody(data);
  if (!body) return fallback;

  if (body.status === 'SOLD_OUT') return 'Товара больше нет';

  if (body.error === QUEUE_LIMIT_REACHED) {
    const limit = typeof body.limit === 'number' && Number.isFinite(body.limit) ? body.limit : null;

    if (limit !== null) {
      return `Нельзя стоять больше чем в ${limit} очередях одновременно. Выйдите из одной, чтобы встать в новую.`;
    }

    return 'Нельзя стоять в таком количестве очередей одновременно. Выйдите из одной, чтобы встать в новую.';
  }

  return fallback;
};
