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
import { z } from 'zod'

import type { AdminUserInvitationPayload, UserInvitationRecord } from '../types'

export const userInvitationFormSchema = z.object({
  quotaAmount: z.number().min(0),
  group: z.string().min(1),
  planId: z.string(),
  remark: z.string().max(255),
  expiresInDays: z.number().int().min(1).max(30),
})

export type UserInvitationFormValues = z.infer<typeof userInvitationFormSchema>

export const USER_INVITATION_FORM_DEFAULTS: UserInvitationFormValues = {
  quotaAmount: 0,
  group: 'default',
  planId: '',
  remark: '',
  expiresInDays: 7,
}

export function buildUserInvitationPayload(
  values: UserInvitationFormValues,
  now: number,
  quotaConverter: (amount: number) => number
): AdminUserInvitationPayload {
  return {
    quota: quotaConverter(values.quotaAmount),
    group: values.group,
    plan_id: values.planId ? Number(values.planId) : 0,
    remark: values.remark.trim(),
    expires_at: now + values.expiresInDays * 86_400,
  }
}

export function invitationRecordToFormValues(
  record: UserInvitationRecord,
  now: number,
  quotaConverter: (quota: number) => number
): UserInvitationFormValues {
  const secondsRemaining = Math.max(record.invitation.expires_at - now, 86_400)
  return {
    quotaAmount: quotaConverter(record.entitlements.quota),
    group: record.entitlements.group,
    planId: record.entitlements.plan
      ? String(record.entitlements.plan.plan_id)
      : '',
    remark: record.invitation.remark ?? '',
    expiresInDays: Math.min(
      30,
      Math.max(1, Math.ceil(secondsRemaining / 86_400))
    ),
  }
}
