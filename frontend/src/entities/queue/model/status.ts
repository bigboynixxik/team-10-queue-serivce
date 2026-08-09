import type { MembershipStatus } from '../api/type';

const terminalStatuses = new Set<MembershipStatus>(['DECLINED', 'PURCHASED', 'SOLD_OUT']);

/** статусы после которых live-обновления уже не нужны */
export const isTerminalStatus = (status?: MembershipStatus): boolean =>
  Boolean(status && terminalStatuses.has(status));
