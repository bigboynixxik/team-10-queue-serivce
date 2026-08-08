export const readEnv = (value: unknown, fallback: string): string =>
  typeof value === 'string' && value.trim() ? value.trim() : fallback;

export const readEnvNumber = (value: unknown, fallback: number): number =>
  typeof value === 'string' && value.trim() ? Number.parseInt(value.trim(), 10) : fallback;

export const readEnvBoolean = (value: unknown, fallback: boolean): boolean =>
  typeof value === 'string' && value.trim() ? value.trim() === 'true' : fallback;
