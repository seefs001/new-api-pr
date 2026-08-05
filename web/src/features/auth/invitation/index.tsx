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
import { Loading03Icon } from '@hugeicons/core-free-icons'
import { HugeiconsIcon } from '@hugeicons/react'
import { useQuery } from '@tanstack/react-query'
import { useState, type ReactNode } from 'react'
import { useForm } from 'react-hook-form'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import type { z } from 'zod'

import { PasswordInput } from '@/components/password-input'
import { Turnstile } from '@/components/turnstile'
import { Button } from '@/components/ui/button'
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from '@/components/ui/card'
import {
  Form,
  FormControl,
  FormField,
  FormItem,
  FormLabel,
  FormMessage,
} from '@/components/ui/form'
import { Input } from '@/components/ui/input'
import {
  InputGroup,
  InputGroupAddon,
  InputGroupButton,
  InputGroupInput,
} from '@/components/ui/input-group'
import { Label } from '@/components/ui/label'
import { Skeleton } from '@/components/ui/skeleton'
import { formatDuration } from '@/features/subscriptions/lib'
import { useStatus } from '@/hooks/use-status'
import { formatQuota } from '@/lib/format'

import { claimInvitation, getInvitationPreview } from '../api'
import { AuthLayout } from '../auth-layout'
import { LegalConsent } from '../components/legal-consent'
import { TermsFooter } from '../components/terms-footer'
import { registerFormSchema } from '../constants'
import { useAuthRedirect } from '../hooks/use-auth-redirect'
import { useEmailVerification } from '../hooks/use-email-verification'
import { useTurnstile } from '../hooks/use-turnstile'

type InvitationClaimProps = {
  token: string
}

export function InvitationClaim(props: InvitationClaimProps) {
  const { t } = useTranslation()
  const isManual = props.token === 'manual'
  const [manualCode, setManualCode] = useState('')
  const [credential, setCredential] = useState(isManual ? '' : props.token)
  const [verificationCode, setVerificationCode] = useState('')
  const [agreedToLegal, setAgreedToLegal] = useState(false)
  const [isSubmitting, setIsSubmitting] = useState(false)
  const [turnstileWidgetKey, setTurnstileWidgetKey] = useState(0)
  const { status } = useStatus()
  const { redirectToLogin } = useAuthRedirect()
  const {
    isTurnstileEnabled,
    turnstileSiteKey,
    turnstileToken,
    setTurnstileToken,
    validateTurnstile,
  } = useTurnstile()
  const {
    isSending: isSendingCode,
    secondsLeft,
    isActive,
    sendCode,
  } = useEmailVerification({ turnstileToken, validateTurnstile })

  const credentialType = isManual ? 'code' : 'link'
  const previewQuery = useQuery({
    queryKey: ['user-invitation-preview', credentialType, credential],
    queryFn: () => getInvitationPreview(credentialType, credential),
    enabled: credential.length > 0,
    retry: false,
  })

  const form = useForm<z.infer<typeof registerFormSchema>>({
    resolver: zodResolver(registerFormSchema),
    defaultValues: {
      username: '',
      email: '',
      password: '',
      confirmPassword: '',
    },
  })

  const preview = previewQuery.data?.data
  const emailValue = form.watch('email')
  const emailVerificationRequired = Boolean(status?.email_verification)
  const requiresLegalConsent = Boolean(
    status?.user_agreement_enabled || status?.privacy_policy_enabled
  )
  const turnstileReady = !isTurnstileEnabled || Boolean(turnstileToken)

  const handleLoadManualCode = () => {
    const normalized = manualCode.trim()
    if (!normalized) {
      toast.error(t('Enter your invitation code'))
      return
    }
    setCredential(normalized)
  }

  const handleSendVerificationCode = async () => {
    if (await sendCode(emailValue || '')) {
      setTurnstileToken('')
      setTurnstileWidgetKey((current) => current + 1)
    }
  }

  const onSubmit = async (values: z.infer<typeof registerFormSchema>) => {
    if (!preview || !credential) {
      return
    }
    if (requiresLegalConsent && !agreedToLegal) {
      toast.error(t('Please agree to the legal terms first'))
      return
    }
    if (emailVerificationRequired && (!values.email || !verificationCode)) {
      toast.error(t('Please enter your email and verification code'))
      return
    }
    if (!validateTurnstile()) {
      return
    }

    setIsSubmitting(true)
    try {
      const response = await claimInvitation({
        credential_type: credentialType,
        credential,
        username: values.username,
        password: values.password,
        email: emailVerificationRequired ? values.email : undefined,
        verification_code: emailVerificationRequired
          ? verificationCode
          : undefined,
        turnstile: turnstileToken,
      })
      if (!response.success) {
        toast.error(response.message || t('Failed to claim invitation'))
        return
      }
      toast.success(t('Invitation claimed! Please sign in'))
      redirectToLogin()
    } catch {
      toast.error(t('An unexpected error occurred'))
    } finally {
      setIsSubmitting(false)
    }
  }

  let verificationCodeAction: ReactNode = t('Send code')
  if (isActive) {
    verificationCodeAction = t('Resend ({{seconds}}s)', {
      seconds: secondsLeft,
    })
  } else if (isSendingCode) {
    verificationCodeAction = (
      <HugeiconsIcon icon={Loading03Icon} className='animate-spin' />
    )
  }

  return (
    <AuthLayout>
      <div className='w-full space-y-6'>
        <div className='space-y-2'>
          <h2 className='text-2xl font-semibold tracking-tight'>
            {t('Claim invitation')}
          </h2>
          <p className='text-muted-foreground text-sm'>
            {t('Review your benefits and create your account.')}
          </p>
        </div>

        {isManual && !preview && (
          <div className='grid gap-2'>
            <Label htmlFor='manual-invitation-code'>
              {t('Manual claim code')}
            </Label>
            <InputGroup>
              <InputGroupInput
                id='manual-invitation-code'
                value={manualCode}
                onChange={(event) => setManualCode(event.target.value)}
                placeholder={t('Enter your invitation code')}
                autoComplete='one-time-code'
              />
              <InputGroupAddon align='inline-end'>
                <InputGroupButton onClick={handleLoadManualCode}>
                  {t('Continue')}
                </InputGroupButton>
              </InputGroupAddon>
            </InputGroup>
          </div>
        )}

        {credential && previewQuery.isLoading && (
          <div className='space-y-3'>
            <Skeleton className='h-24 w-full' />
            <Skeleton className='h-48 w-full' />
          </div>
        )}

        {credential &&
          (previewQuery.isError ||
            (previewQuery.data && !previewQuery.data.success)) && (
            <Card>
              <CardHeader>
                <CardTitle>{t('Invalid or unavailable invitation')}</CardTitle>
                <CardDescription>
                  {previewQuery.data?.message ||
                    t('An unexpected error occurred')}
                </CardDescription>
              </CardHeader>
            </Card>
          )}

        {preview && (
          <>
            <Card>
              <CardHeader>
                <CardTitle>{t('Invitation benefits')}</CardTitle>
                <CardDescription>
                  {t(
                    'These benefits are applied to your account when you claim the invitation.'
                  )}
                </CardDescription>
              </CardHeader>
              <CardContent className='grid gap-3 sm:grid-cols-2'>
                <div>
                  <p className='text-muted-foreground text-xs'>
                    {t('Wallet quota')}
                  </p>
                  <p className='font-medium'>
                    {formatQuota(preview.entitlements.quota)}
                  </p>
                </div>
                <div>
                  <p className='text-muted-foreground text-xs'>
                    {t('Base group')}
                  </p>
                  <p className='font-medium'>{preview.entitlements.group}</p>
                </div>
                {preview.entitlements.plan && (
                  <>
                    <div>
                      <p className='text-muted-foreground text-xs'>
                        {t('Plan')}
                      </p>
                      <p className='font-medium'>
                        {preview.entitlements.plan.title}
                      </p>
                    </div>
                    <div>
                      <p className='text-muted-foreground text-xs'>
                        {t('Subscription quota')}
                      </p>
                      <p className='font-medium'>
                        {formatQuota(preview.entitlements.plan.total_amount)}
                      </p>
                    </div>
                    <div>
                      <p className='text-muted-foreground text-xs'>
                        {t('Duration')}
                      </p>
                      <p className='font-medium'>
                        {formatDuration(preview.entitlements.plan, t)}
                      </p>
                    </div>
                  </>
                )}
              </CardContent>
            </Card>

            <Form {...form}>
              <form
                onSubmit={form.handleSubmit(onSubmit)}
                className='grid gap-4'
              >
                <FormField
                  control={form.control}
                  name='username'
                  render={({ field }) => (
                    <FormItem>
                      <FormLabel>{t('Username')}</FormLabel>
                      <FormControl>
                        <Input
                          placeholder={t('Enter your username')}
                          {...field}
                        />
                      </FormControl>
                      <FormMessage />
                    </FormItem>
                  )}
                />
                <FormField
                  control={form.control}
                  name='password'
                  render={({ field }) => (
                    <FormItem>
                      <FormLabel>{t('Password')}</FormLabel>
                      <FormControl>
                        <PasswordInput
                          placeholder={t('Enter password (8-20 characters)')}
                          {...field}
                        />
                      </FormControl>
                      <FormMessage />
                    </FormItem>
                  )}
                />
                <FormField
                  control={form.control}
                  name='confirmPassword'
                  render={({ field }) => (
                    <FormItem>
                      <FormLabel>{t('Confirm password')}</FormLabel>
                      <FormControl>
                        <PasswordInput
                          placeholder={t('Confirm password')}
                          {...field}
                        />
                      </FormControl>
                      <FormMessage />
                    </FormItem>
                  )}
                />

                {emailVerificationRequired && (
                  <>
                    <FormField
                      control={form.control}
                      name='email'
                      render={({ field }) => (
                        <FormItem>
                          <FormLabel>
                            {t('Email (required for verification)')}
                          </FormLabel>
                          <FormControl>
                            <Input
                              type='email'
                              placeholder={t('name@example.com')}
                              {...field}
                            />
                          </FormControl>
                          <FormMessage />
                        </FormItem>
                      )}
                    />
                    <div className='grid gap-2'>
                      <Label htmlFor='invitation-verification-code'>
                        {t('Verification code')}
                      </Label>
                      <InputGroup>
                        <InputGroupInput
                          id='invitation-verification-code'
                          value={verificationCode}
                          onChange={(event) =>
                            setVerificationCode(event.target.value)
                          }
                        />
                        <InputGroupAddon align='inline-end'>
                          <InputGroupButton
                            variant='outline'
                            disabled={
                              isSubmitting ||
                              isSendingCode ||
                              isActive ||
                              !emailValue ||
                              !turnstileReady
                            }
                            onClick={handleSendVerificationCode}
                          >
                            {verificationCodeAction}
                          </InputGroupButton>
                        </InputGroupAddon>
                      </InputGroup>
                    </div>
                  </>
                )}

                {isTurnstileEnabled && (
                  <Turnstile
                    key={turnstileWidgetKey}
                    siteKey={turnstileSiteKey}
                    onVerify={setTurnstileToken}
                  />
                )}

                <LegalConsent
                  status={status}
                  checked={agreedToLegal}
                  onCheckedChange={setAgreedToLegal}
                />

                <Button
                  type='submit'
                  disabled={
                    isSubmitting ||
                    !turnstileReady ||
                    (requiresLegalConsent && !agreedToLegal)
                  }
                >
                  {isSubmitting && (
                    <HugeiconsIcon
                      icon={Loading03Icon}
                      className='animate-spin'
                      data-icon='inline-start'
                    />
                  )}
                  {t('Claim invitation')}
                </Button>
              </form>
            </Form>
          </>
        )}

        <TermsFooter
          variant='sign-up'
          status={status}
          className='text-center'
        />
      </div>
    </AuthLayout>
  )
}
