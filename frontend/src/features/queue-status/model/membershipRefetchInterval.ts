import { isTerminalStatus, type MembershipStatus } from '@entities/queue';

/** polling только пока есть живой membership; без статуса (404 / ещё не в очереди) не долбим */
export const membershipRefetchInterval = (status?: MembershipStatus): false | 2000 =>
  status && !isTerminalStatus(status) ? 2000 : false;
