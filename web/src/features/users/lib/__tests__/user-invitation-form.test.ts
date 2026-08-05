/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/
import assert from 'node:assert/strict'
import { describe, test } from 'node:test'

import { buildUserInvitationPayload } from '../user-invitation-form'

describe('buildUserInvitationPayload', () => {
  test('converts the displayed quota and expiry days into the admin API contract', () => {
    const now = 1_800_000_000

    assert.deepEqual(
      buildUserInvitationPayload(
        {
          quotaAmount: 2.5,
          group: 'vip',
          planId: '12',
          remark: 'partner',
          expiresInDays: 7,
        },
        now,
        (amount) => amount * 500_000
      ),
      {
        quota: 1_250_000,
        group: 'vip',
        plan_id: 12,
        remark: 'partner',
        expires_at: now + 7 * 86_400,
      }
    )
  })
})
