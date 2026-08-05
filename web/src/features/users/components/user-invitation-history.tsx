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
import { useTranslation } from 'react-i18next'

import { SideDrawerSection } from '@/components/drawer-layout'
import { StatusBadge } from '@/components/status-badge'

import { USER_STATUSES, USER_STATUS } from '../constants'
import type { UserInvitationRecord } from '../types'

const INVITATION_STATUS_LABELS = {
  active: 'Active',
  claimed: 'Claimed',
  revoked: 'Revoked',
  expired: 'Expired',
} as const

type UserInvitationHistoryProps = {
  records: UserInvitationRecord[]
}

export function UserInvitationHistory(props: UserInvitationHistoryProps) {
  const { t } = useTranslation()
  if (props.records.length === 0) {
    return null
  }

  return (
    <SideDrawerSection>
      <h3 className='text-sm font-medium'>{t('Invitation history')}</h3>
      <div className='space-y-2'>
        {props.records.map((record) => (
          <div
            key={record.invitation.id}
            className='bg-muted/40 flex items-center justify-between rounded-lg px-3 py-2 text-sm'
          >
            <span>#{record.invitation.id}</span>
            <StatusBadge
              label={t(INVITATION_STATUS_LABELS[record.invitation.status])}
              variant={
                record.invitation.status === 'active'
                  ? USER_STATUSES[USER_STATUS.PENDING_CLAIM].variant
                  : 'neutral'
              }
              copyable={false}
            />
          </div>
        ))}
      </div>
    </SideDrawerSection>
  )
}
