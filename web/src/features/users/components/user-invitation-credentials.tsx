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
import { Copy01Icon } from '@hugeicons/core-free-icons'
import { HugeiconsIcon } from '@hugeicons/react'
import { useTranslation } from 'react-i18next'

import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert'
import {
  InputGroup,
  InputGroupAddon,
  InputGroupButton,
  InputGroupInput,
} from '@/components/ui/input-group'
import { Label } from '@/components/ui/label'
import { useCopyToClipboard } from '@/hooks/use-copy-to-clipboard'

import type { UserInvitationIssue } from '../types'

type UserInvitationCredentialsProps = {
  issue: UserInvitationIssue
}

export function UserInvitationCredentials(
  props: UserInvitationCredentialsProps
) {
  const { t } = useTranslation()
  const { copyToClipboard } = useCopyToClipboard()
  const invitationUrl = `${window.location.origin}/invite/${props.issue.link_token}`

  return (
    <Alert className='border-success/40'>
      <AlertTitle>{t('Invitation created')}</AlertTitle>
      <AlertDescription className='space-y-4'>
        <p>{t('Copy this link and code now. They will not be shown again.')}</p>
        <div className='grid gap-2'>
          <Label htmlFor='invitation-link'>{t('Invitation link')}</Label>
          <InputGroup>
            <InputGroupInput
              id='invitation-link'
              value={invitationUrl}
              readOnly
            />
            <InputGroupAddon align='inline-end'>
              <InputGroupButton
                size='icon-xs'
                aria-label={t('Copy invitation link')}
                onClick={() => copyToClipboard(invitationUrl)}
              >
                <HugeiconsIcon icon={Copy01Icon} />
              </InputGroupButton>
            </InputGroupAddon>
          </InputGroup>
        </div>
        <div className='grid gap-2'>
          <Label htmlFor='invitation-code'>{t('Manual claim code')}</Label>
          <InputGroup>
            <InputGroupInput
              id='invitation-code'
              value={props.issue.manual_code}
              readOnly
            />
            <InputGroupAddon align='inline-end'>
              <InputGroupButton
                size='icon-xs'
                aria-label={t('Copy invitation code')}
                onClick={() => copyToClipboard(props.issue.manual_code)}
              >
                <HugeiconsIcon icon={Copy01Icon} />
              </InputGroupButton>
            </InputGroupAddon>
          </InputGroup>
        </div>
      </AlertDescription>
    </Alert>
  )
}
