import { isTerminalStatus, type MembershipStatus } from '@entities/queue';

/** polling только пока membership ещё живой */
export const membershipRefetchInterval = (status?: MembershipStatus): false | 2000 =>
  isTerminalStatus(status) ? false : 2000;
