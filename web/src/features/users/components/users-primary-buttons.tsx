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
import { Add01Icon, UserAdd01Icon } from '@hugeicons/core-free-icons'
import { HugeiconsIcon } from '@hugeicons/react'
import { useTranslation } from 'react-i18next'

import { Button } from '@/components/ui/button'

import { useUsers } from './users-provider'

export function UsersPrimaryButtons() {
  const { t } = useTranslation()
  const { setOpen, setCurrentRow } = useUsers()

  const handleCreate = () => {
    setCurrentRow(null)
    setOpen('create')
  }

  const handleInvite = () => {
    setCurrentRow(null)
    setOpen('invite')
  }

  return (
    <div className='flex gap-2'>
      <Button size='sm' variant='outline' onClick={handleInvite}>
        <HugeiconsIcon icon={UserAdd01Icon} data-icon='inline-start' />
        {t('Invite User')}
      </Button>
      <Button size='sm' onClick={handleCreate}>
        <HugeiconsIcon icon={Add01Icon} data-icon='inline-start' />
        {t('Add User')}
      </Button>
    </div>
  )
}
