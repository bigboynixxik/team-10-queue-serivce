import type { MembershipStatus } from '../api/type';

const terminalStatuses = new Set<MembershipStatus>(['DECLINED', 'PURCHASED', 'SOLD_OUT']);

export const isTerminalStatus = (status?: MembershipStatus): boolean =>
  Boolean(status && terminalStatuses.has(status));
