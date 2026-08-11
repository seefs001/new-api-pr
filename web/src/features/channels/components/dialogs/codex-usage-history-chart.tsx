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
import { useQuery } from '@tanstack/react-query'
import { RefreshCw } from 'lucide-react'
import { useMemo, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { CartesianGrid, Legend, Line, LineChart, XAxis, YAxis } from 'recharts'

import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert'
import { Button } from '@/components/ui/button'
import {
  ChartContainer,
  ChartLegendContent,
  ChartTooltip,
  ChartTooltipContent,
  type ChartConfig,
} from '@/components/ui/chart'
import {
  Empty,
  EmptyDescription,
  EmptyHeader,
  EmptyTitle,
} from '@/components/ui/empty'
import {
  Select,
  SelectContent,
  SelectGroup,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { Skeleton } from '@/components/ui/skeleton'
import { Tabs, TabsList, TabsTrigger } from '@/components/ui/tabs'
import dayjs from '@/lib/dayjs'
import { formatTimestampToDate } from '@/lib/format'

import {
  getCodexUsageHistory,
  type CodexUsageHistoryData,
  type CodexUsageHistoryLimit,
} from '../../api'
import { buildCodexUsageChartData } from '../../lib/codex-usage-history'

type HistoryRange = '24h' | '7d' | '30d' | '90d'

const HISTORY_RANGES: Array<{ value: HistoryRange; label: string }> = [
  { value: '24h', label: '24 Hours' },
  { value: '7d', label: '7 Days' },
  { value: '30d', label: '30 Days' },
  { value: '90d', label: '90 Days' },
]

const CHART_COLORS = [
  'var(--chart-1)',
  'var(--chart-2)',
  'var(--chart-3)',
  'var(--chart-4)',
] as const

type CodexUsageHistoryChartProps = {
  channelId: number
}

function formatWindowLabel(
  limit: CodexUsageHistoryLimit,
  t: (key: string) => string
): string {
  if (limit.window_seconds === 5 * 60 * 60) return t('5-Hour Window')
  if (limit.window_seconds === 7 * 24 * 60 * 60) return t('Weekly Window')
  if (limit.window_seconds && limit.window_seconds > 0) {
    const hours = limit.window_seconds / 3600
    return Number.isInteger(hours)
      ? `${hours}${t('h')}`
      : `${Math.round(hours * 10) / 10}${t('h')}`
  }
  if (limit.window_key === 'slot:primary') return t('Primary window')
  if (limit.window_key === 'slot:secondary') return t('Secondary window')
  return limit.window_key
}

function UsageLineChart(props: {
  limits: CodexUsageHistoryLimit[]
  range: HistoryRange
  ariaLabel: string
}) {
  const { t } = useTranslation()
  const chartData = useMemo(
    () => buildCodexUsageChartData(props.limits),
    [props.limits]
  )
  const chartConfig = useMemo<ChartConfig>(() => {
    const config: ChartConfig = {}
    chartData.series.forEach((series, index) => {
      const limit = props.limits[index]
      const windowLabel = formatWindowLabel(limit, t)
      config[series.dataKey] = {
        label:
          limit.limit_type === 'additional'
            ? `${limit.limit_key} · ${windowLabel}`
            : windowLabel,
        color: CHART_COLORS[index % CHART_COLORS.length],
      }
    })
    return config
  }, [chartData.series, props.limits, t])

  if (chartData.series.length === 0 || chartData.rows.length === 0) return null

  return (
    <ChartContainer
      config={chartConfig}
      className='aspect-auto h-[280px] w-full'
      role='img'
      aria-label={props.ariaLabel}
    >
      <LineChart
        data={chartData.rows}
        margin={{ top: 12, right: 16, bottom: 4, left: 0 }}
        accessibilityLayer
      >
        <CartesianGrid vertical={false} />
        <XAxis
          dataKey='timestamp'
          type='number'
          domain={['dataMin', 'dataMax']}
          tickLine={false}
          axisLine={false}
          tickMargin={8}
          tickFormatter={(value: number) =>
            dayjs.unix(value).format(props.range === '24h' ? 'HH:mm' : 'MM-DD')
          }
        />
        <YAxis
          domain={[0, 100]}
          width={42}
          tickLine={false}
          axisLine={false}
          tickFormatter={(value: number) => `${value}%`}
        />
        <ChartTooltip
          content={
            <ChartTooltipContent
              labelFormatter={(_, payload) => {
                const timestamp = Number(payload[0]?.payload?.timestamp)
                return Number.isFinite(timestamp)
                  ? formatTimestampToDate(timestamp)
                  : '-'
              }}
              formatter={(value, name) => (
                <div className='flex min-w-36 flex-1 items-center justify-between gap-4'>
                  <span className='text-muted-foreground'>
                    {chartConfig[String(name)]?.label ?? String(name)}
                  </span>
                  <span className='font-mono font-medium tabular-nums'>
                    {Number(value).toFixed(1)}%
                  </span>
                </div>
              )}
            />
          }
        />
        <Legend content={<ChartLegendContent />} />
        {chartData.series.map((series) => (
          <Line
            key={series.dataKey}
            dataKey={series.dataKey}
            name={series.dataKey}
            type='stepAfter'
            stroke={`var(--color-${series.dataKey})`}
            strokeWidth={2}
            dot={false}
            activeDot={{ r: 4 }}
            connectNulls={false}
            isAnimationActive={false}
          />
        ))}
      </LineChart>
    </ChartContainer>
  )
}

function HistoryContent(props: {
  data: CodexUsageHistoryData
  range: HistoryRange
}) {
  const { t } = useTranslation()
  const [selectedAdditionalLimit, setSelectedAdditionalLimit] = useState('')
  const baseLimits = props.data.limits.filter(
    (limit) => limit.limit_type === 'base'
  )
  const additionalKeys = [
    ...new Set(
      props.data.limits
        .filter((limit) => limit.limit_type === 'additional')
        .map((limit) => limit.limit_key)
    ),
  ]
  const activeAdditionalLimit = additionalKeys.includes(selectedAdditionalLimit)
    ? selectedAdditionalLimit
    : (additionalKeys[0] ?? '')
  const additionalLimits = props.data.limits.filter(
    (limit) =>
      limit.limit_type === 'additional' &&
      limit.limit_key === activeAdditionalLimit
  )

  if (props.data.limits.length === 0) {
    return (
      <Empty className='min-h-52 border'>
        <EmptyHeader>
          <EmptyTitle>{t('No historical usage yet')}</EmptyTitle>
          <EmptyDescription>
            {props.data.collection.enabled
              ? t('Waiting for the first Codex usage sample.')
              : t('Codex usage history collection is disabled.')}
          </EmptyDescription>
        </EmptyHeader>
      </Empty>
    )
  }

  return (
    <div className='space-y-4'>
      {baseLimits.length > 0 ? (
        <div className='rounded-lg border p-2'>
          <UsageLineChart
            limits={baseLimits}
            range={props.range}
            ariaLabel={t('Base limit usage history')}
          />
        </div>
      ) : null}

      {additionalKeys.length > 0 ? (
        <div className='space-y-3 rounded-lg border p-3'>
          <div className='flex flex-wrap items-center justify-between gap-2'>
            <div className='text-sm font-medium'>{t('Additional Limits')}</div>
            <Select
              items={additionalKeys.map((key) => ({ value: key, label: key }))}
              value={activeAdditionalLimit}
              onValueChange={(value) => setSelectedAdditionalLimit(value ?? '')}
            >
              <SelectTrigger className='w-full sm:w-64'>
                <SelectValue />
              </SelectTrigger>
              <SelectContent alignItemWithTrigger={false}>
                <SelectGroup>
                  {additionalKeys.map((key) => (
                    <SelectItem key={key} value={key}>
                      {key}
                    </SelectItem>
                  ))}
                </SelectGroup>
              </SelectContent>
            </Select>
          </div>
          <UsageLineChart
            limits={additionalLimits}
            range={props.range}
            ariaLabel={t('Additional limit usage history')}
          />
        </div>
      ) : null}
    </div>
  )
}

export default function CodexUsageHistoryChart(
  props: CodexUsageHistoryChartProps
) {
  const { t } = useTranslation()
  const [range, setRange] = useState<HistoryRange>('7d')
  const query = useQuery({
    queryKey: ['channels', 'codex-usage-history', props.channelId, range],
    queryFn: async () => {
      const response = await getCodexUsageHistory(props.channelId, range)
      if (!response.success || !response.data) {
        throw new Error(response.message || t('Failed to fetch usage history'))
      }
      return response.data
    },
    staleTime: 60_000,
  })

  return (
    <section className='space-y-3'>
      <div className='flex flex-wrap items-start justify-between gap-3'>
        <div>
          <div className='text-sm font-semibold'>{t('Historical Usage')}</div>
          <div className='text-muted-foreground mt-1 text-xs leading-5'>
            {t(
              'Sampled Codex upstream usage; gaps indicate missing observations.'
            )}
          </div>
          {query.data?.collection.last_observed_at ? (
            <div className='text-muted-foreground mt-1 text-xs'>
              {t('Last updated:')}{' '}
              {formatTimestampToDate(query.data.collection.last_observed_at)}
            </div>
          ) : null}
        </div>
        <div className='flex max-w-full items-center gap-2 overflow-x-auto pb-1'>
          <Tabs
            value={range}
            onValueChange={(value) => setRange(value as HistoryRange)}
          >
            <TabsList>
              {HISTORY_RANGES.map((item) => (
                <TabsTrigger key={item.value} value={item.value}>
                  {t(item.label)}
                </TabsTrigger>
              ))}
            </TabsList>
          </Tabs>
          <Button
            type='button'
            variant='outline'
            size='icon-sm'
            aria-label={t('Refresh')}
            onClick={() => void query.refetch()}
            disabled={query.isFetching}
          >
            <RefreshCw className={query.isFetching ? 'animate-spin' : ''} />
          </Button>
        </div>
      </div>

      {query.data && !query.data.collection.enabled ? (
        <Alert>
          <AlertTitle>{t('Usage history collection is paused')}</AlertTitle>
          <AlertDescription>
            {t(
              'Enable Codex usage history in Monitoring & Alerts to collect new samples.'
            )}
          </AlertDescription>
        </Alert>
      ) : null}

      {query.data?.collection.stale ? (
        <Alert>
          <AlertTitle>{t('Usage history may be stale')}</AlertTitle>
          <AlertDescription>
            {t('No recent Codex usage sample has been recorded.')}
          </AlertDescription>
        </Alert>
      ) : null}

      {query.isLoading ? <Skeleton className='h-[320px] w-full' /> : null}
      {query.isError ? (
        <Alert variant='destructive'>
          <AlertTitle>{t('Failed to fetch usage history')}</AlertTitle>
          <AlertDescription>
            {query.error instanceof Error
              ? query.error.message
              : t('Unknown error')}
          </AlertDescription>
        </Alert>
      ) : null}
      {query.data ? <HistoryContent data={query.data} range={range} /> : null}
    </section>
  )
}
