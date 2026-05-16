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
import { type ReactNode, useMemo, useState } from 'react'
import {
  AlertTriangle,
  Check,
  ChevronDown,
  ChevronUp,
  Clock3,
  Copy,
  Gauge,
  Hash,
  Mail,
  RefreshCw,
  ShieldCheck,
  User,
} from 'lucide-react'
import { useTranslation } from 'react-i18next'
import dayjs from '@/lib/dayjs'
import { cn } from '@/lib/utils'
import { useCopyToClipboard } from '@/hooks/use-copy-to-clipboard'
import { Button } from '@/components/ui/button'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import { Progress } from '@/components/ui/progress'
import { ScrollArea } from '@/components/ui/scroll-area'
import {
  Tooltip,
  TooltipContent,
  TooltipProvider,
  TooltipTrigger,
} from '@/components/ui/tooltip'
import { StatusBadge, type StatusBadgeProps } from '@/components/status-badge'

type CodexRateLimitWindow = {
  used_percent?: unknown
  reset_at?: unknown
  reset_after_seconds?: unknown
  limit_window_seconds?: unknown
}

type CodexRateLimit = {
  plan_type?: unknown
  allowed?: boolean
  limit_reached?: boolean
  primary_window?: unknown
  secondary_window?: unknown
}

type CodexAdditionalRateLimit = {
  limit_name?: unknown
  metered_feature?: unknown
  rate_limit?: unknown
  primary_window?: unknown
  secondary_window?: unknown
  plan_type?: unknown
}

type CodexUsagePayload = {
  plan_type?: unknown
  user_id?: unknown
  email?: unknown
  account_id?: unknown
  rate_limit?: unknown
  primary_window?: unknown
  secondary_window?: unknown
  additional_rate_limits?: unknown
}

export type CodexUsageDialogData = {
  success: boolean
  message?: string
  upstream_status?: number
  data?: Record<string, unknown>
}

type CodexUsageDialogProps = {
  open: boolean
  onOpenChange: (open: boolean) => void
  channelName?: string
  channelId?: number
  response: CodexUsageDialogData | null
  onRefresh?: () => void
  isRefreshing?: boolean
}

type RateLimitSource = {
  plan_type?: unknown
  rate_limit?: unknown
  primary_window?: unknown
  secondary_window?: unknown
} | null

type LimitWindow = {
  key: 'fiveHour' | 'weekly'
  label: string
  window: CodexRateLimitWindow | null
}

type LimitItem = {
  id: string
  label: string
  kindLabel: string
  kindVariant: StatusBadgeProps['variant']
  meteredFeature: string
  windows: LimitWindow[]
  maxPercent: number
}

type WindowEntry = {
  itemLabel: string
  windowLabel: string
  window: CodexRateLimitWindow
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return !!value && typeof value === 'object' && !Array.isArray(value)
}

function getDisplayText(value: unknown): string {
  if (value == null) return ''
  return String(value).trim()
}

function asRateLimit(value: unknown): CodexRateLimit | null {
  return isRecord(value) ? (value as CodexRateLimit) : null
}

function asRateLimitWindow(value: unknown): CodexRateLimitWindow | null {
  return isRecord(value) ? (value as CodexRateLimitWindow) : null
}

function clampPercent(value: unknown): number {
  const v = Number(value)
  return Number.isFinite(v) ? Math.max(0, Math.min(100, v)) : 0
}

function formatPercent(value: unknown): string {
  const rounded = Math.round(clampPercent(value) * 10) / 10
  return Number.isInteger(rounded) ? String(rounded) : rounded.toFixed(1)
}

function formatUnixSeconds(unixSeconds: unknown): string {
  const v = Number(unixSeconds)
  if (!Number.isFinite(v) || v <= 0) return '-'
  try {
    return dayjs(v * 1000).format('YYYY-MM-DD HH:mm:ss')
  } catch {
    return getDisplayText(unixSeconds) || '-'
  }
}

function formatDurationSeconds(
  seconds: unknown,
  t: (key: string) => string
): string {
  const s = Number(seconds)
  if (!Number.isFinite(s) || s <= 0) return '-'

  const total = Math.floor(s)
  const hours = Math.floor(total / 3600)
  const minutes = Math.floor((total % 3600) / 60)
  const secs = total % 60

  if (hours > 0) return `${hours}${t('h')} ${minutes}${t('m')}`
  if (minutes > 0) return `${minutes}${t('m')} ${secs}${t('s')}`
  return `${secs}${t('s')}`
}

function normalizePlanType(value: unknown): string {
  return getDisplayText(value).toLowerCase()
}

function classifyWindowByDuration(
  windowData?: CodexRateLimitWindow | null
): 'weekly' | 'fiveHour' | null {
  const seconds = Number(windowData?.limit_window_seconds)
  if (!Number.isFinite(seconds) || seconds <= 0) return null
  return seconds >= 24 * 60 * 60 ? 'weekly' : 'fiveHour'
}

function hasWindowData(
  windowData?: CodexRateLimitWindow | null
): windowData is CodexRateLimitWindow {
  return isRecord(windowData) && Object.keys(windowData).length > 0
}

function resolveRateLimitWindows(data: RateLimitSource): {
  fiveHourWindow: CodexRateLimitWindow | null
  weeklyWindow: CodexRateLimitWindow | null
} {
  const rateLimit = asRateLimit(data?.rate_limit)
  const primary =
    asRateLimitWindow(rateLimit?.primary_window) ??
    asRateLimitWindow(data?.primary_window)
  const secondary =
    asRateLimitWindow(rateLimit?.secondary_window) ??
    asRateLimitWindow(data?.secondary_window)
  const windows = [primary, secondary].filter(Boolean) as CodexRateLimitWindow[]
  const planType = normalizePlanType(data?.plan_type ?? rateLimit?.plan_type)

  let fiveHourWindow: CodexRateLimitWindow | null = null
  let weeklyWindow: CodexRateLimitWindow | null = null

  for (const w of windows) {
    const bucket = classifyWindowByDuration(w)
    if (bucket === 'fiveHour' && !fiveHourWindow) {
      fiveHourWindow = w
      continue
    }
    if (bucket === 'weekly' && !weeklyWindow) {
      weeklyWindow = w
    }
  }

  if (planType === 'free') {
    if (!weeklyWindow) weeklyWindow = primary ?? secondary ?? null
    return { fiveHourWindow: null, weeklyWindow }
  }

  if (!fiveHourWindow && !weeklyWindow) {
    return { fiveHourWindow: primary, weeklyWindow: secondary }
  }

  if (!fiveHourWindow) {
    fiveHourWindow = windows.find((w) => w !== weeklyWindow) ?? null
  }
  if (!weeklyWindow) {
    weeklyWindow = windows.find((w) => w !== fiveHourWindow) ?? null
  }

  return { fiveHourWindow, weeklyWindow }
}

const PLAN_TYPE_BADGE: Record<
  string,
  { label: string; variant: StatusBadgeProps['variant'] }
> = {
  enterprise: { label: 'Enterprise', variant: 'success' },
  team: { label: 'Team', variant: 'info' },
  pro: { label: 'Pro', variant: 'blue' },
  plus: { label: 'Plus', variant: 'purple' },
  free: { label: 'Free', variant: 'warning' },
}

function getAccountTypeBadge(
  value: unknown,
  t: (key: string) => string
): { label: string; variant: StatusBadgeProps['variant'] } {
  const normalized = normalizePlanType(value)
  return (
    PLAN_TYPE_BADGE[normalized] ?? {
      label: getDisplayText(value) || t('Unknown'),
      variant: 'neutral' as const,
    }
  )
}

function getUsageVariant(percent: number): StatusBadgeProps['variant'] {
  if (percent >= 95) return 'danger'
  if (percent >= 80) return 'warning'
  return 'info'
}

function getStatusConfig(
  rateLimit: CodexRateLimit | null,
  failed: boolean,
  t: (key: string) => string
): { label: string; variant: StatusBadgeProps['variant'] } {
  if (failed) {
    return { label: t('Failed'), variant: 'danger' }
  }
  if (!rateLimit || Object.keys(rateLimit).length === 0) {
    return { label: t('Pending'), variant: 'neutral' }
  }
  if (rateLimit.allowed === true && rateLimit.limit_reached !== true) {
    return { label: t('Available'), variant: 'success' }
  }
  return { label: t('Limited'), variant: 'danger' }
}

function getResetAfterSeconds(
  windowData?: CodexRateLimitWindow | null
): number | null {
  const resetAfter = Number(windowData?.reset_after_seconds)
  if (Number.isFinite(resetAfter) && resetAfter > 0) return resetAfter

  const resetAt = Number(windowData?.reset_at)
  if (!Number.isFinite(resetAt) || resetAt <= 0) return null

  const seconds = resetAt - Math.floor(Date.now() / 1000)
  return seconds > 0 ? seconds : null
}

function createLimitItem(params: {
  id: string
  label: string
  kindLabel: string
  kindVariant: StatusBadgeProps['variant']
  meteredFeature?: string
  source: RateLimitSource
  fiveHourLabel: string
  weeklyLabel: string
}): LimitItem {
  const { fiveHourWindow, weeklyWindow } = resolveRateLimitWindows(params.source)
  const windows: LimitWindow[] = [
    { key: 'fiveHour', label: params.fiveHourLabel, window: fiveHourWindow },
    { key: 'weekly', label: params.weeklyLabel, window: weeklyWindow },
  ]
  const maxPercent = windows.reduce((max, item) => {
    if (!hasWindowData(item.window)) return max
    return Math.max(max, clampPercent(item.window.used_percent))
  }, 0)

  return {
    id: params.id,
    label: params.label,
    kindLabel: params.kindLabel,
    kindVariant: params.kindVariant,
    meteredFeature: params.meteredFeature ?? '',
    windows,
    maxPercent,
  }
}

function collectWindowEntries(items: LimitItem[]): WindowEntry[] {
  const entries: WindowEntry[] = []

  for (const item of items) {
    for (const windowItem of item.windows) {
      if (!hasWindowData(windowItem.window)) continue
      entries.push({
        itemLabel: item.label,
        windowLabel: windowItem.label,
        window: windowItem.window,
      })
    }
  }

  return entries
}

function getUsageSummary(items: LimitItem[]): {
  highest: WindowEntry | null
  nextReset: WindowEntry | null
} {
  const entries = collectWindowEntries(items)
  const highest = entries.reduce<WindowEntry | null>((current, entry) => {
    if (!current) return entry
    return clampPercent(entry.window.used_percent) >
      clampPercent(current.window.used_percent)
      ? entry
      : current
  }, null)
  const nextReset = entries.reduce<WindowEntry | null>((current, entry) => {
    const resetAfter = getResetAfterSeconds(entry.window)
    if (resetAfter == null) return current
    if (!current) return entry

    const currentResetAfter = getResetAfterSeconds(current.window)
    return currentResetAfter == null || resetAfter < currentResetAfter
      ? entry
      : current
  }, null)

  return { highest, nextReset }
}

function SummaryCell(props: {
  icon: ReactNode
  label: string
  children: ReactNode
}) {
  return (
    <div className='bg-popover min-w-0 p-3'>
      <div className='text-muted-foreground flex items-center gap-1.5 text-xs'>
        <span className='[&_svg]:size-3.5'>{props.icon}</span>
        <span className='truncate'>{props.label}</span>
      </div>
      <div className='mt-1 min-w-0 text-sm font-medium'>{props.children}</div>
    </div>
  )
}

function UsageMeter(props: {
  title: string
  windowData?: CodexRateLimitWindow | null
}) {
  const { t } = useTranslation()

  if (!hasWindowData(props.windowData)) {
    return (
      <div className='bg-muted/30 min-w-0 rounded-md p-2.5'>
        <div className='flex items-center justify-between gap-2'>
          <span className='truncate text-xs font-medium'>{props.title}</span>
          <StatusBadge
            label={t('Unknown')}
            variant='neutral'
            copyable={false}
          />
        </div>
      </div>
    )
  }

  const percent = clampPercent(props.windowData.used_percent)
  const variant = getUsageVariant(percent)

  return (
    <div className='bg-muted/30 min-w-0 rounded-md p-2.5'>
      <div className='flex items-center justify-between gap-2'>
        <span className='truncate text-xs font-medium'>{props.title}</span>
        <StatusBadge
          label={`${formatPercent(percent)}%`}
          variant={variant}
          copyable={false}
        />
      </div>
      <Progress
        value={percent}
        aria-label={`${props.title} usage: ${formatPercent(percent)}%`}
        className={cn(
          'mt-2',
          variant === 'danger' &&
            '[&_[data-slot=progress-indicator]]:bg-destructive',
          variant === 'warning' &&
            '[&_[data-slot=progress-indicator]]:bg-warning'
        )}
      />
      <div className='text-muted-foreground mt-2 flex min-w-0 flex-wrap gap-x-3 gap-y-1 text-xs'>
        <span className='whitespace-nowrap'>
          {t('Resets in:')}{' '}
          {formatDurationSeconds(props.windowData.reset_after_seconds, t)}
        </span>
        <span className='min-w-0 truncate'>
          {t('Reset at:')} {formatUnixSeconds(props.windowData.reset_at)}
        </span>
      </div>
    </div>
  )
}

function LimitRow(props: { item: LimitItem }) {
  return (
    <div className='grid min-w-0 gap-3 p-3 sm:grid-cols-[minmax(0,1.15fr)_minmax(0,1fr)_minmax(0,1fr)] sm:items-center'>
      <div className='min-w-0 space-y-1'>
        <div className='flex min-w-0 flex-wrap items-center gap-2'>
          <span
            className='min-w-0 max-w-full truncate text-sm font-medium'
            title={props.item.label}
          >
            {props.item.label}
          </span>
          <StatusBadge
            label={props.item.kindLabel}
            variant={props.item.kindVariant}
            copyable={false}
          />
        </div>
        {props.item.meteredFeature && (
          <div
            className='text-muted-foreground max-w-full truncate font-mono text-xs'
            title={props.item.meteredFeature}
          >
            {props.item.meteredFeature}
          </div>
        )}
      </div>
      {props.item.windows.map((item) => (
        <UsageMeter
          key={`${props.item.id}-${item.key}`}
          title={item.label}
          windowData={item.window}
        />
      ))}
    </div>
  )
}

function CopyableField(props: {
  icon: ReactNode
  label: string
  value?: unknown
  mono?: boolean
}) {
  const { t } = useTranslation()
  const { copyToClipboard, copiedText } = useCopyToClipboard({ notify: false })
  const text = getDisplayText(props.value)
  const hasCopied = copiedText === text

  const copyButton = (
    <Button
      type='button'
      variant='ghost'
      size='icon-sm'
      className='justify-self-end'
      onClick={() => copyToClipboard(text)}
      disabled={!text}
      aria-label={`${t('Copy')} ${props.label}`}
    >
      {hasCopied ? (
        <Check className='size-3.5 text-green-600' />
      ) : (
        <Copy className='size-3.5' />
      )}
    </Button>
  )

  return (
    <div className='grid min-w-0 grid-cols-[1rem_minmax(4.5rem,6.5rem)_minmax(0,1fr)_2rem] items-center gap-2 py-2 text-sm'>
      <span className='text-muted-foreground flex justify-center'>
        {props.icon}
      </span>
      <span className='text-muted-foreground truncate text-xs'>
        {props.label}
      </span>
      <span
        className={cn(
          'min-w-0 truncate text-xs',
          props.mono && 'font-mono'
        )}
        title={text || undefined}
      >
        {text || '-'}
      </span>
      {text ? (
        <Tooltip>
          <TooltipTrigger render={copyButton}></TooltipTrigger>
          <TooltipContent>
            <p>{hasCopied ? t('Copied') : t('Copy')}</p>
          </TooltipContent>
        </Tooltip>
      ) : (
        <span />
      )}
    </div>
  )
}

function DisclosureSection(props: {
  title: string
  open: boolean
  onToggle: () => void
  children: ReactNode
}) {
  return (
    <div className='overflow-hidden rounded-lg border'>
      <button
        type='button'
        className='hover:bg-muted/40 flex w-full items-center justify-between gap-3 p-3 text-left transition-colors'
        onClick={props.onToggle}
        aria-expanded={props.open}
      >
        <span className='min-w-0 truncate text-sm font-medium'>
          {props.title}
        </span>
        {props.open ? (
          <ChevronUp className='text-muted-foreground size-4 shrink-0' />
        ) : (
          <ChevronDown className='text-muted-foreground size-4 shrink-0' />
        )}
      </button>
      {props.open && <div className='border-t p-3'>{props.children}</div>}
    </div>
  )
}

export function CodexUsageDialog({
  open,
  onOpenChange,
  channelName,
  channelId,
  response,
  onRefresh,
  isRefreshing,
}: CodexUsageDialogProps) {
  const { t } = useTranslation()
  const { copiedText, copyToClipboard } = useCopyToClipboard({ notify: false })
  const [showAccountDetails, setShowAccountDetails] = useState(false)
  const [showRawJson, setShowRawJson] = useState(false)

  const payload: CodexUsagePayload | null = useMemo(() => {
    const raw = response?.data
    if (!isRecord(raw)) return null
    return raw as CodexUsagePayload
  }, [response?.data])

  const rateLimit = asRateLimit(payload?.rate_limit)
  const accountType = payload?.plan_type ?? rateLimit?.plan_type
  const accountBadge = getAccountTypeBadge(accountType, t)
  const rawAdditionalRateLimits = Array.isArray(payload?.additional_rate_limits)
    ? payload.additional_rate_limits
    : []
  const additionalRateLimits = rawAdditionalRateLimits.filter(
    (item): item is CodexAdditionalRateLimit =>
      isRecord(item) && Object.keys(item).length > 0
  )

  const baseLimitItem = createLimitItem({
    id: 'base',
    label: t('Base Limits'),
    kindLabel: t('Base Limits'),
    kindVariant: 'blue',
    source: payload,
    fiveHourLabel: t('5-Hour Window'),
    weeklyLabel: t('Weekly Window'),
  })
  const additionalLimitItems = additionalRateLimits
    .map((item, index) => {
      const meteredFeature = getDisplayText(item.metered_feature)
      const label =
        getDisplayText(item.limit_name) ||
        meteredFeature ||
        `${t('Additional Limit')} ${index + 1}`

      return createLimitItem({
        id: `additional-${meteredFeature || label}-${index}`,
        label,
        kindLabel: t('Additional Limits'),
        kindVariant: 'purple',
        meteredFeature,
        source: item,
        fiveHourLabel: t('5-Hour Window'),
        weeklyLabel: t('Weekly Window'),
      })
    })
    .sort((a, b) => b.maxPercent - a.maxPercent)
  const limitItems = [baseLimitItem, ...additionalLimitItems]
  const usageSummary = getUsageSummary(limitItems)
  const statusConfig = getStatusConfig(
    rateLimit,
    response?.success === false,
    t
  )

  const errorMessage =
    response?.success === false
      ? getDisplayText(response?.message) || t('Failed to fetch usage')
      : ''

  const rawJsonText = useMemo(() => {
    if (!response) return ''
    try {
      return JSON.stringify(
        {
          success: response.success,
          message: response.message,
          upstream_status: response.upstream_status,
          data: response.data,
        },
        null,
        2
      )
    } catch {
      return getDisplayText(response?.data)
    }
  }, [response])

  const nextResetSeconds = usageSummary.nextReset
    ? getResetAfterSeconds(usageSummary.nextReset.window)
    : null

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className='flex max-h-[calc(100dvh-1rem)] w-[calc(100vw-1rem)] max-w-[58rem] flex-col gap-0 overflow-hidden p-0 sm:max-h-[calc(100dvh-2rem)] sm:w-[calc(100vw-2rem)] sm:max-w-[58rem]'>
        <TooltipProvider delay={150}>
          <DialogHeader className='border-b p-4 pr-12'>
            <DialogTitle>{t('Codex Account & Usage')}</DialogTitle>
            <DialogDescription className='min-w-0 truncate'>
              {t('Channel:')} <strong>{channelName || '-'}</strong>{' '}
              {channelId ? `(#${channelId})` : ''}
            </DialogDescription>
          </DialogHeader>

          <ScrollArea className='min-h-0 flex-1'>
            <div className='space-y-4 p-4'>
              {errorMessage && (
                <div className='border-destructive/30 bg-destructive/10 text-destructive flex items-start gap-2 rounded-lg border px-3 py-2 text-sm'>
                  <AlertTriangle className='mt-0.5 size-4 shrink-0' />
                  <span className='min-w-0 break-words'>{errorMessage}</span>
                </div>
              )}

              <div className='grid grid-cols-2 gap-px overflow-hidden rounded-lg border bg-border lg:grid-cols-4'>
                <SummaryCell icon={<ShieldCheck />} label={t('Status')}>
                  <StatusBadge
                    label={statusConfig.label}
                    variant={statusConfig.variant}
                    copyable={false}
                  />
                </SummaryCell>
                <SummaryCell icon={<User />} label={t('Plan')}>
                  <StatusBadge
                    label={accountBadge.label}
                    variant={accountBadge.variant}
                    copyable={false}
                  />
                </SummaryCell>
                <SummaryCell icon={<Gauge />} label={t('Usage')}>
                  {usageSummary.highest ? (
                    <div className='min-w-0'>
                      <span>
                        {formatPercent(
                          usageSummary.highest.window.used_percent
                        )}
                        %
                      </span>
                      <div className='text-muted-foreground truncate text-xs font-normal'>
                        {usageSummary.highest.itemLabel} ·{' '}
                        {usageSummary.highest.windowLabel}
                      </div>
                    </div>
                  ) : (
                    <span className='text-muted-foreground'>-</span>
                  )}
                </SummaryCell>
                <SummaryCell icon={<Clock3 />} label={t('Next reset')}>
                  {usageSummary.nextReset && nextResetSeconds != null ? (
                    <div className='min-w-0'>
                      <span>{formatDurationSeconds(nextResetSeconds, t)}</span>
                      <div className='text-muted-foreground truncate text-xs font-normal'>
                        {usageSummary.nextReset.itemLabel} ·{' '}
                        {usageSummary.nextReset.windowLabel}
                      </div>
                    </div>
                  ) : (
                    <span className='text-muted-foreground'>-</span>
                  )}
                </SummaryCell>
              </div>

              <section className='space-y-2'>
                <div className='flex min-w-0 flex-wrap items-center justify-between gap-2'>
                  <h3 className='text-sm font-medium'>
                    {t('Rate Limit Windows')}
                  </h3>
                  {typeof response?.upstream_status === 'number' && (
                    <StatusBadge
                      label={`HTTP ${response.upstream_status}`}
                      variant='neutral'
                      copyable={false}
                    />
                  )}
                </div>
                <div className='divide-y overflow-hidden rounded-lg border'>
                  {limitItems.map((item) => (
                    <LimitRow key={item.id} item={item} />
                  ))}
                </div>
              </section>

              <DisclosureSection
                title={t('Account Info')}
                open={showAccountDetails}
                onToggle={() => setShowAccountDetails((value) => !value)}
              >
                <div className='divide-y'>
                  <CopyableField
                    icon={<User className='size-3.5' />}
                    label='User ID'
                    value={payload?.user_id}
                    mono
                  />
                  <CopyableField
                    icon={<Mail className='size-3.5' />}
                    label={t('Email')}
                    value={payload?.email}
                  />
                  <CopyableField
                    icon={<Hash className='size-3.5' />}
                    label='Account ID'
                    value={payload?.account_id}
                    mono
                  />
                  {typeof response?.upstream_status === 'number' && (
                    <CopyableField
                      icon={<Hash className='size-3.5' />}
                      label='HTTP'
                      value={response.upstream_status}
                      mono
                    />
                  )}
                </div>
              </DisclosureSection>

              <DisclosureSection
                title={t('Raw JSON')}
                open={showRawJson}
                onToggle={() => setShowRawJson((value) => !value)}
              >
                <div className='mb-2 flex justify-end'>
                  <Button
                    type='button'
                    variant='outline'
                    size='sm'
                    onClick={() => copyToClipboard(rawJsonText)}
                    disabled={!rawJsonText}
                  >
                    {copiedText === rawJsonText ? (
                      <Check className='mr-1.5 size-3.5 text-green-600' />
                    ) : (
                      <Copy className='mr-1.5 size-3.5' />
                    )}
                    {t('Copy')}
                  </Button>
                </div>
                <ScrollArea className='max-h-[min(45vh,24rem)] rounded-md border'>
                  <pre className='bg-muted/30 m-0 p-3 text-xs break-words whitespace-pre-wrap'>
                    {rawJsonText || '-'}
                  </pre>
                </ScrollArea>
              </DisclosureSection>
            </div>
          </ScrollArea>

          <DialogFooter className='mx-0 mb-0 rounded-none border-t bg-background p-3'>
            {onRefresh && (
              <Button
                type='button'
                variant='outline'
                size='sm'
                onClick={onRefresh}
                disabled={Boolean(isRefreshing)}
                className='w-full sm:w-auto'
              >
                <RefreshCw
                  className={cn('mr-1.5 size-3.5', isRefreshing && 'animate-spin')}
                />
                {t('Refresh')}
              </Button>
            )}
            <Button
              type='button'
              variant='outline'
              size='sm'
              onClick={() => onOpenChange(false)}
              className='w-full sm:w-auto'
            >
              {t('Close')}
            </Button>
          </DialogFooter>
        </TooltipProvider>
      </DialogContent>
    </Dialog>
  )
}
