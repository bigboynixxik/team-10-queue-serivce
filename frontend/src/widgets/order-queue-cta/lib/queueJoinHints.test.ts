import { describe, expect, test } from '@rstest/core';

import { QUEUE_JOIN_HINTS } from './queueJoinHints';

describe('queueJoinHints', () => {
  test('exposes join guidance copy', () => {
    expect(QUEUE_JOIN_HINTS).toHaveLength(2);
    expect(QUEUE_JOIN_HINTS[0]?.title).toBeTruthy();
    expect(QUEUE_JOIN_HINTS[1]?.description).toContain('право на оплату');
  });
});
