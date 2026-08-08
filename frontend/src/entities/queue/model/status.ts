import type { MembershipStatus } from '../api/type';

const terminalStatuses = new Set<MembershipStatus>(['DECLINED', 'PURCHASED', 'SOLD_OUT']);

/**
 * A terminal membership is history: the queue is over for this user, so it must
 * not be offered as something to return to.
 */
export const isTerminalStatus = (status?: MembershipStatus): boolean =>
  Boolean(status && terminalStatuses.has(status));
