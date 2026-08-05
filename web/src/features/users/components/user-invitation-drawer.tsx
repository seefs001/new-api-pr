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
import { zodResolver } from '@hookform/resolvers/zod'
import { useQuery } from '@tanstack/react-query'
import { useEffect, useMemo, useState } from 'react'
import { useForm } from 'react-hook-form'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import {
  SideDrawerSection,
  sideDrawerContentClassName,
  sideDrawerFooterClassName,
  sideDrawerFormClassName,
  sideDrawerHeaderClassName,
} from '@/components/drawer-layout'
import { Button } from '@/components/ui/button'
import {
  Form,
  FormControl,
  FormDescription,
  FormField,
  FormItem,
  FormLabel,
  FormMessage,
} from '@/components/ui/form'
import { Input } from '@/components/ui/input'
import {
  Select,
  SelectContent,
  SelectGroup,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import {
  Sheet,
  SheetContent,
  SheetDescription,
  SheetFooter,
  SheetHeader,
  SheetTitle,
} from '@/components/ui/sheet'
import { Textarea } from '@/components/ui/textarea'
import { getAdminPlans } from '@/features/subscriptions/api'
import {
  formatQuota,
  parseQuotaFromDollars,
  quotaUnitsToDollars,
} from '@/lib/format'

import {
  createUserInvitation,
  getGroups,
  getUserInvitations,
  reissueUserInvitation,
  revokeUserInvitation,
  updateUserInvitation,
} from '../api'
import {
  buildUserInvitationPayload,
  invitationRecordToFormValues,
  USER_INVITATION_FORM_DEFAULTS,
  userInvitationFormSchema,
  type UserInvitationFormValues,
} from '../lib/user-invitation-form'
import type { User, UserInvitationIssue } from '../types'
import { UserInvitationCredentials } from './user-invitation-credentials'
import { UserInvitationHistory } from './user-invitation-history'
import { useUsers } from './users-provider'

type UserInvitationDrawerProps = {
  open: boolean
  onOpenChange: (open: boolean) => void
  currentRow?: User
}

type SubmitMode = 'create' | 'update' | 'reissue'
export function UserInvitationDrawer(props: UserInvitationDrawerProps) {
  const { t } = useTranslation()
  const { triggerRefresh } = useUsers()
  const [isSubmitting, setIsSubmitting] = useState(false)
  const [issue, setIssue] = useState<UserInvitationIssue | null>(null)
  const isManaging = Boolean(props.currentRow)

  const groupsQuery = useQuery({
    queryKey: ['groups'],
    queryFn: getGroups,
    enabled: props.open,
    staleTime: 5 * 60 * 1000,
  })
  const plansQuery = useQuery({
    queryKey: ['subscription-plans', 'admin'],
    queryFn: getAdminPlans,
    enabled: props.open,
    staleTime: 60 * 1000,
  })
  const invitationsQuery = useQuery({
    queryKey: ['user-invitations', props.currentRow?.id],
    queryFn: () => getUserInvitations(props.currentRow?.id ?? 0),
    enabled: props.open && Boolean(props.currentRow),
  })

  const records = invitationsQuery.data?.data ?? []
  const activeRecord = records.find(
    (record) => record.invitation.status === 'active'
  )
  const latestRecord = activeRecord ?? records[0]

  const form = useForm<UserInvitationFormValues>({
    resolver: zodResolver(userInvitationFormSchema),
    defaultValues: USER_INVITATION_FORM_DEFAULTS,
  })

  useEffect(() => {
    if (props.open) {
      setIssue(null)
    }
  }, [props.currentRow?.id, props.open])

  useEffect(() => {
    if (!props.open || issue) {
      return
    }
    if (props.currentRow && latestRecord) {
      form.reset(
        invitationRecordToFormValues(
          latestRecord,
          Math.floor(Date.now() / 1000),
          quotaUnitsToDollars
        )
      )
      return
    }
    if (!props.currentRow) {
      form.reset(USER_INVITATION_FORM_DEFAULTS)
    }
  }, [form, issue, latestRecord, props.currentRow, props.open])

  const groupItems = useMemo(
    () =>
      (groupsQuery.data?.data ?? []).map((group) => ({
        value: group,
        label: group,
      })),
    [groupsQuery.data?.data]
  )
  const planItems = useMemo(() => {
    const items = (plansQuery.data?.data ?? [])
      .filter((record) => record.plan.enabled)
      .map((record) => ({
        value: String(record.plan.id),
        label: record.plan.title,
      }))
    const promisedPlan = latestRecord?.entitlements.plan
    if (
      promisedPlan &&
      !items.some((item) => item.value === String(promisedPlan.plan_id))
    ) {
      items.unshift({
        value: String(promisedPlan.plan_id),
        label: promisedPlan.title,
      })
    }
    return [{ value: '', label: t('No subscription') }, ...items]
  }, [latestRecord, plansQuery.data?.data, t])

  const submitInvitation = async (
    values: UserInvitationFormValues,
    mode: SubmitMode
  ) => {
    const currentRow = props.currentRow
    if (mode !== 'create' && !currentRow) {
      return
    }

    setIsSubmitting(true)
    try {
      const payload = buildUserInvitationPayload(
        values,
        Math.floor(Date.now() / 1000),
        parseQuotaFromDollars
      )
      let response
      if (mode === 'create') {
        response = await createUserInvitation(payload)
      } else if (mode === 'update' && currentRow) {
        response = await updateUserInvitation(currentRow.id, payload)
      } else if (currentRow) {
        response = await reissueUserInvitation(currentRow.id, payload)
      } else {
        return
      }
      if (!response.success) {
        toast.error(response.message || t('Failed to save invitation'))
        return
      }
      if (mode !== 'update' && response.data) {
        setIssue(response.data as UserInvitationIssue)
      }
      toast.success(
        mode === 'update' ? t('Invitation updated') : t('Invitation created')
      )
      triggerRefresh()
      if (props.currentRow) {
        await invitationsQuery.refetch()
      }
    } catch {
      toast.error(t('An unexpected error occurred'))
    } finally {
      setIsSubmitting(false)
    }
  }

  const handleRevoke = async () => {
    const currentRow = props.currentRow
    if (!currentRow) {
      return
    }
    setIsSubmitting(true)
    try {
      const response = await revokeUserInvitation(currentRow.id)
      if (!response.success) {
        toast.error(response.message || t('Failed to revoke invitation'))
        return
      }
      toast.success(t('Invitation revoked'))
      triggerRefresh()
      await invitationsQuery.refetch()
    } catch {
      toast.error(t('An unexpected error occurred'))
    } finally {
      setIsSubmitting(false)
    }
  }

  let submitMode: SubmitMode = 'create'
  if (isManaging) {
    submitMode = activeRecord ? 'update' : 'reissue'
  }

  return (
    <Sheet open={props.open} onOpenChange={props.onOpenChange}>
      <SheetContent className={sideDrawerContentClassName('sm:max-w-[620px]')}>
        <SheetHeader className={sideDrawerHeaderClassName()}>
          <SheetTitle>
            {isManaging ? t('Manage invitation') : t('Invite User')}
          </SheetTitle>
          <SheetDescription>
            {t('Create an invitation without entering user credentials.')}
          </SheetDescription>
        </SheetHeader>

        <Form {...form}>
          <form
            id='user-invitation-form'
            onSubmit={form.handleSubmit((values) =>
              submitInvitation(values, submitMode)
            )}
            className={sideDrawerFormClassName()}
          >
            {issue && <UserInvitationCredentials issue={issue} />}

            <SideDrawerSection>
              <h3 className='text-sm font-medium'>
                {t('Invitation benefits')}
              </h3>
              <FormField
                control={form.control}
                name='quotaAmount'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>{t('Initial quota')}</FormLabel>
                    <FormControl>
                      <Input
                        type='number'
                        min={0}
                        step='0.01'
                        {...field}
                        onChange={(event) =>
                          field.onChange(event.target.valueAsNumber)
                        }
                      />
                    </FormControl>
                    <FormDescription>
                      {formatQuota(parseQuotaFromDollars(field.value || 0))}
                    </FormDescription>
                    <FormMessage />
                  </FormItem>
                )}
              />

              <FormField
                control={form.control}
                name='group'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>{t('Base group')}</FormLabel>
                    <Select
                      items={groupItems}
                      value={field.value}
                      onValueChange={field.onChange}
                    >
                      <FormControl>
                        <SelectTrigger>
                          <SelectValue />
                        </SelectTrigger>
                      </FormControl>
                      <SelectContent>
                        <SelectGroup>
                          {groupItems.map((item) => (
                            <SelectItem key={item.value} value={item.value}>
                              {item.label}
                            </SelectItem>
                          ))}
                        </SelectGroup>
                      </SelectContent>
                    </Select>
                    <FormMessage />
                  </FormItem>
                )}
              />

              <FormField
                control={form.control}
                name='planId'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>{t('Subscription')}</FormLabel>
                    <Select
                      items={planItems}
                      value={field.value}
                      onValueChange={field.onChange}
                    >
                      <FormControl>
                        <SelectTrigger>
                          <SelectValue />
                        </SelectTrigger>
                      </FormControl>
                      <SelectContent>
                        <SelectGroup>
                          {planItems.map((item) => (
                            <SelectItem
                              key={item.value || 'none'}
                              value={item.value}
                            >
                              {item.label}
                            </SelectItem>
                          ))}
                        </SelectGroup>
                      </SelectContent>
                    </Select>
                    <FormDescription>
                      {t(
                        'The subscription starts when the invitation is claimed.'
                      )}
                    </FormDescription>
                    <FormMessage />
                  </FormItem>
                )}
              />
            </SideDrawerSection>

            <SideDrawerSection>
              <h3 className='text-sm font-medium'>
                {t('Invitation settings')}
              </h3>
              <FormField
                control={form.control}
                name='expiresInDays'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>{t('Expiration (days)')}</FormLabel>
                    <FormControl>
                      <Input
                        type='number'
                        min={1}
                        max={30}
                        {...field}
                        onChange={(event) =>
                          field.onChange(event.target.valueAsNumber)
                        }
                      />
                    </FormControl>
                    <FormDescription>
                      {t('Invitations can remain valid for up to 30 days.')}
                    </FormDescription>
                    <FormMessage />
                  </FormItem>
                )}
              />
              <FormField
                control={form.control}
                name='remark'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>{t('Internal note')}</FormLabel>
                    <FormControl>
                      <Textarea
                        rows={3}
                        placeholder={t('Only administrators can see this note')}
                        {...field}
                      />
                    </FormControl>
                    <FormMessage />
                  </FormItem>
                )}
              />
            </SideDrawerSection>

            {isManaging && <UserInvitationHistory records={records} />}
          </form>
        </Form>

        <SheetFooter className={sideDrawerFooterClassName()}>
          {isManaging && activeRecord && (
            <>
              <Button
                type='button'
                variant='destructive'
                onClick={handleRevoke}
                disabled={isSubmitting || Boolean(issue)}
              >
                {t('Revoke invitation')}
              </Button>
              <Button
                type='button'
                variant='outline'
                onClick={form.handleSubmit((values) =>
                  submitInvitation(values, 'reissue')
                )}
                disabled={isSubmitting || Boolean(issue)}
              >
                {t('Reissue invitation')}
              </Button>
            </>
          )}
          <Button
            type='submit'
            form='user-invitation-form'
            disabled={isSubmitting || Boolean(issue)}
          >
            {submitMode === 'create' && t('Create invitation')}
            {submitMode === 'update' && t('Update invitation')}
            {submitMode === 'reissue' && t('Reissue invitation')}
          </Button>
        </SheetFooter>
      </SheetContent>
    </Sheet>
  )
}
