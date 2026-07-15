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
import {
  AlertTriangle,
  Check,
  ChevronDown,
  ChevronUp,
  Copy,
  RefreshCw,
} from 'lucide-react'
import { useMemo, useState } from 'react'
import { useTranslation } from 'react-i18next'

import { Dialog } from '@/components/dialog'
import { StatusBadge } from '@/components/status-badge'
import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert'
import { Button } from '@/components/ui/button'
import {
  Card,
  CardAction,
  CardContent,
  CardHeader,
  CardTitle,
} from '@/components/ui/card'
import {
  Collapsible,
  CollapsibleContent,
  CollapsibleTrigger,
} from '@/components/ui/collapsible'
import { Progress } from '@/components/ui/progress'
import { ScrollArea } from '@/components/ui/scroll-area'
import { Skeleton } from '@/components/ui/skeleton'
import { useCopyToClipboard } from '@/hooks/use-copy-to-clipboard'
import { toIntlLocale } from '@/i18n/languages'

import type { GrokUsageResponse, GrokUsageWindow } from '../../api'

type GrokUsageDialogProps = {
  open: boolean
  onOpenChange: (open: boolean) => void
  channelName?: string
  channelId?: number
  channelDisplayName?: string
  channelDisplayId?: string
  response: GrokUsageResponse | null
  onRefresh?: () => void | Promise<void>
  isRefreshing?: boolean
}

function asRecord(value: unknown): Record<string, unknown> | null {
  return value !== null && typeof value === 'object' && !Array.isArray(value)
    ? (value as Record<string, unknown>)
    : null
}

function getBillingConfig(window?: GrokUsageWindow) {
  return asRecord(asRecord(window?.data)?.config)
}

function getNumber(value: unknown): number | null {
  const raw = asRecord(value)?.val ?? value
  const number = Number(raw)
  return Number.isFinite(number) ? number : null
}

function clampPercent(value: unknown): number | null {
  const number = getNumber(value)
  return number === null ? null : Math.min(100, Math.max(0, number))
}

function formatPeriodDate(value: unknown, locale: string): string {
  if (typeof value !== 'string' || !value) return '-'
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return value
  return new Intl.DateTimeFormat(locale, {
    dateStyle: 'medium',
    timeStyle: 'short',
  }).format(date)
}

function UsagePeriod({
  start,
  end,
  locale,
}: {
  start: unknown
  end: unknown
  locale: string
}) {
  const { t } = useTranslation()
  return (
    <div className='text-muted-foreground grid gap-1 text-xs sm:grid-cols-2'>
      <div>
        {t('Start')}: {formatPeriodDate(start, locale)}
      </div>
      <div>
        {t('End')}: {formatPeriodDate(end, locale)}
      </div>
    </div>
  )
}

export function GrokUsageDialog({
  open,
  onOpenChange,
  channelName,
  channelId,
  channelDisplayName,
  channelDisplayId,
  response,
  onRefresh,
  isRefreshing = false,
}: GrokUsageDialogProps) {
  const { t, i18n } = useTranslation()
  const [showRawJson, setShowRawJson] = useState(false)
  const { copiedText, copyToClipboard } = useCopyToClipboard()
  const locale = toIntlLocale(i18n.resolvedLanguage || i18n.language) ?? 'en-US'
  const weeklyConfig = getBillingConfig(response?.data?.weekly)
  const monthlyConfig = getBillingConfig(response?.data?.monthly)
  const currentPeriod = asRecord(weeklyConfig?.currentPeriod)
  const weeklyPercent = clampPercent(weeklyConfig?.creditUsagePercent) ?? 0
  const monthlyLimit = getNumber(monthlyConfig?.monthlyLimit)
  const monthlyUsed = getNumber(monthlyConfig?.used)
  const monthlyPercent =
    monthlyLimit !== null && monthlyLimit > 0 && monthlyUsed !== null
      ? Math.min(100, Math.max(0, (monthlyUsed / monthlyLimit) * 100))
      : 0
  const onDemandCap =
    getNumber(weeklyConfig?.onDemandCap) ??
    getNumber(monthlyConfig?.onDemandCap)
  const onDemandUsed =
    getNumber(weeklyConfig?.onDemandUsed) ??
    getNumber(monthlyConfig?.onDemandUsed)
  const products = Array.isArray(weeklyConfig?.productUsage)
    ? weeklyConfig.productUsage
        .map(asRecord)
        .filter((item): item is Record<string, unknown> => item !== null)
    : []
  let plan = t('Unknown')
  if (monthlyLimit === 15_000) {
    plan = 'SuperGrok'
  } else if (monthlyLimit === 150_000) {
    plan = 'SuperGrok Heavy'
  }
  const formatCents = (value: number | null) =>
    value === null
      ? '-'
      : new Intl.NumberFormat(locale, {
          style: 'currency',
          currency: 'USD',
        }).format(value / 100)
  const channelLabel = `${channelDisplayName ?? channelName ?? '-'} (#${channelDisplayId ?? channelId ?? '-'})`
  const rawJsonText = useMemo(
    () => (response?.data ? JSON.stringify(response.data, null, 2) : ''),
    [response?.data]
  )

  return (
    <Dialog
      open={open}
      onOpenChange={onOpenChange}
      title={`${t('Grok')} · ${t('Account Info')}`}
      description={channelLabel}
      contentHeight='min(68vh, 680px)'
      bodyClassName='flex flex-col gap-4'
      footer={
        <>
          {onRefresh ? (
            <Button
              type='button'
              variant='outline'
              onClick={() => onRefresh()}
              disabled={isRefreshing}
            >
              <RefreshCw
                data-icon='inline-start'
                className={isRefreshing ? 'animate-spin' : undefined}
              />
              {isRefreshing ? t('Refreshing...') : t('Refresh')}
            </Button>
          ) : null}
          <Button type='button' onClick={() => onOpenChange(false)}>
            {t('Close')}
          </Button>
        </>
      }
    >
      {!response ? (
        <div className='grid gap-4 sm:grid-cols-2'>
          <Skeleton className='h-44 rounded-xl' />
          <Skeleton className='h-44 rounded-xl' />
        </div>
      ) : (
        <>
          {response.data?.partial ? (
            <Alert>
              <AlertTriangle />
              <AlertTitle>{t('Usage')}</AlertTitle>
              <AlertDescription>
                {t('Weekly')}: {response.data.weekly.status_code || '-'} ·{' '}
                {t('Monthly')}: {response.data.monthly.status_code || '-'}
              </AlertDescription>
            </Alert>
          ) : null}

          <div className='grid gap-4 sm:grid-cols-2'>
            <Card size='sm'>
              <CardHeader>
                <CardTitle>{t('Weekly')}</CardTitle>
                <CardAction>
                  <StatusBadge
                    label={`${weeklyPercent.toFixed(1)}%`}
                    variant={weeklyPercent >= 90 ? 'warning' : 'info'}
                    copyable={false}
                  />
                </CardAction>
              </CardHeader>
              <CardContent className='flex flex-col gap-3'>
                <Progress value={weeklyPercent} />
                <UsagePeriod
                  start={
                    currentPeriod?.start ?? weeklyConfig?.billingPeriodStart
                  }
                  end={currentPeriod?.end ?? weeklyConfig?.billingPeriodEnd}
                  locale={locale}
                />
                {products.map((item) => {
                  const product = String(item.product ?? t('Product'))
                  const percent = clampPercent(item.usagePercent) ?? 0
                  return (
                    <div key={product} className='flex flex-col gap-1'>
                      <div className='flex justify-between gap-2 text-xs'>
                        <span>{product}</span>
                        <span className='text-muted-foreground'>
                          {percent.toFixed(1)}%
                        </span>
                      </div>
                      <Progress value={percent} />
                    </div>
                  )
                })}
              </CardContent>
            </Card>

            <Card size='sm'>
              <CardHeader>
                <CardTitle>{t('Monthly')}</CardTitle>
                <CardAction>
                  <StatusBadge
                    label={plan}
                    variant='neutral'
                    copyable={false}
                  />
                </CardAction>
              </CardHeader>
              <CardContent className='flex flex-col gap-3'>
                <div className='flex items-baseline justify-between gap-2'>
                  <span className='text-muted-foreground text-xs'>
                    {t('Used')}
                  </span>
                  <span className='font-medium'>
                    {formatCents(monthlyUsed)} / {formatCents(monthlyLimit)}
                  </span>
                </div>
                <Progress value={monthlyPercent} />
                <UsagePeriod
                  start={monthlyConfig?.billingPeriodStart}
                  end={monthlyConfig?.billingPeriodEnd}
                  locale={locale}
                />
                {onDemandCap !== null || onDemandUsed !== null ? (
                  <div className='bg-muted/50 flex items-center justify-between gap-2 rounded-lg border px-3 py-2 text-xs'>
                    <span className='text-muted-foreground'>{t('Extra')}</span>
                    <span>
                      {formatCents(onDemandUsed)} / {formatCents(onDemandCap)}
                    </span>
                  </div>
                ) : null}
              </CardContent>
            </Card>
          </div>

          <Collapsible
            open={showRawJson}
            onOpenChange={setShowRawJson}
            className='rounded-lg border'
          >
            <CollapsibleTrigger
              render={
                <button
                  type='button'
                  className='hover:bg-muted/40 flex w-full items-center justify-between gap-2 p-3 transition-colors'
                  aria-expanded={showRawJson}
                />
              }
            >
              <div className='text-sm font-medium'>{t('Raw JSON')}</div>
              {showRawJson ? (
                <ChevronUp className='text-muted-foreground size-4' />
              ) : (
                <ChevronDown className='text-muted-foreground size-4' />
              )}
            </CollapsibleTrigger>
            <CollapsibleContent>
              <div className='flex justify-end border-t px-3 py-2'>
                <Button
                  type='button'
                  variant='outline'
                  size='sm'
                  onClick={() => copyToClipboard(rawJsonText)}
                  disabled={!rawJsonText}
                >
                  {copiedText === rawJsonText ? (
                    <Check data-icon='inline-start' className='text-success' />
                  ) : (
                    <Copy data-icon='inline-start' />
                  )}
                  {t('Copy')}
                </Button>
              </div>
              <ScrollArea className='max-h-[45vh]'>
                <pre className='bg-muted/30 m-0 p-3 text-xs break-words whitespace-pre-wrap'>
                  {rawJsonText || '-'}
                </pre>
              </ScrollArea>
            </CollapsibleContent>
          </Collapsible>
        </>
      )}
    </Dialog>
  )
}
