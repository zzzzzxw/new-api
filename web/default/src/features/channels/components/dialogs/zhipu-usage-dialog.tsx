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
import { Calendar, Info, RefreshCw } from 'lucide-react'
import { useMemo, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { CartesianGrid, Line, LineChart, XAxis, YAxis } from 'recharts'

import { Dialog } from '@/components/dialog'
import { Button } from '@/components/ui/button'
import {
  Card,
  CardAction,
  CardContent,
  CardHeader,
  CardTitle,
} from '@/components/ui/card'
import {
  ChartContainer,
  ChartTooltip,
  ChartTooltipContent,
  type ChartConfig,
} from '@/components/ui/chart'
import { ScrollArea } from '@/components/ui/scroll-area'
import { cn } from '@/lib/utils'

import type { ZhipuUsageRange, ZhipuUsageResponse } from '../../api'

type ZhipuUsageDialogProps = {
  open: boolean
  onOpenChange: (open: boolean) => void
  channelName: string
  channelId: number
  response: ZhipuUsageResponse | null
  onRefresh?: (range: ZhipuUsageRange) => void | Promise<void>
  isRefreshing?: boolean
}

type QuotaCardData = {
  title: string
  percent: number
  resetTime?: string
}

type UsageSeries = {
  key: string
  label: string
  color: string
  total: number
}

type UsageChartData = {
  points: Array<Record<string, string | number>>
  series: UsageSeries[]
}

const chartColors = ['#2f6bff', '#ff8a3d', '#11a36a', '#8b5cf6']
const usageRangeOptions: Array<{ value: ZhipuUsageRange; label: string }> = [
  { value: 'today', label: 'Today' },
  { value: '7d', label: '7 Days' },
  { value: '30d', label: '30 Days' },
]

function asRecord(value: unknown): Record<string, unknown> | null {
  if (!value || typeof value !== 'object' || Array.isArray(value)) return null
  return value as Record<string, unknown>
}

function toFiniteNumber(value: unknown): number | null {
  if (typeof value === 'number' && Number.isFinite(value)) return value
  if (typeof value === 'string') {
    const number = Number(value.trim().replaceAll(',', ''))
    if (Number.isFinite(number)) return number
  }
  return null
}

function formatCompact(value: unknown) {
  const number = Number(value)
  if (!Number.isFinite(number)) return '-'
  if (Math.abs(number) >= 1_000_000) {
    return `${new Intl.NumberFormat(undefined, {
      maximumFractionDigits: 2,
    }).format(number / 1_000_000)}M`
  }
  if (Math.abs(number) >= 1_000) {
    return `${new Intl.NumberFormat(undefined, {
      maximumFractionDigits: 2,
    }).format(number / 1_000)}K`
  }
  return new Intl.NumberFormat(undefined, { maximumFractionDigits: 2 }).format(
    number
  )
}

function formatDateLabel(value: unknown) {
  if (typeof value !== 'string' && typeof value !== 'number') return ''
  if (typeof value === 'number') {
    const date = new Date(value)
    if (Number.isNaN(date.getTime())) return String(value)
    const month = String(date.getMonth() + 1).padStart(2, '0')
    const day = String(date.getDate()).padStart(2, '0')
    const hour = String(date.getHours()).padStart(2, '0')
    const minute = String(date.getMinutes()).padStart(2, '0')
    return `${date.getFullYear()}-${month}-${day} ${hour}:${minute}`
  }
  const raw = String(value)
  const normalized = raw.includes('T') ? raw : raw.replace(' ', 'T')
  const date = new Date(normalized)
  if (Number.isNaN(date.getTime())) return raw
  const month = String(date.getMonth() + 1).padStart(2, '0')
  const day = String(date.getDate()).padStart(2, '0')
  const hour = String(date.getHours()).padStart(2, '0')
  const minute = String(date.getMinutes()).padStart(2, '0')
  return `${date.getFullYear()}-${month}-${day} ${hour}:${minute}`
}

function formatResetTime(value: unknown) {
  const number = toFiniteNumber(value)
  if (number !== null && number > 1_000_000_000) {
    return formatDateLabel(number < 10_000_000_000 ? number * 1000 : number)
  }
  return formatDateLabel(value)
}

function getUsageRangeDates(range: ZhipuUsageRange) {
  const now = new Date()
  const end = new Date(
    now.getFullYear(),
    now.getMonth(),
    now.getDate(),
    23,
    59,
    59
  )
  const start = new Date(now.getFullYear(), now.getMonth(), now.getDate())
  if (range === '7d') start.setDate(start.getDate() - 6)
  if (range === '30d') start.setDate(start.getDate() - 29)
  return { start, end }
}

function formatRangeDate(value: Date) {
  const month = String(value.getMonth() + 1).padStart(2, '0')
  const day = String(value.getDate()).padStart(2, '0')
  return `${value.getFullYear()}-${month}-${day}`
}

function unwrapEndpoint(response: ZhipuUsageResponse | null, key: string) {
  return response?.data?.endpoints?.[key]?.data
}

function findObjects(value: unknown): Record<string, unknown>[] {
  const record = asRecord(value)
  if (Array.isArray(value)) {
    return value.flatMap((item) => findObjects(item))
  }
  if (!record) return []
  return [record, ...Object.values(record).flatMap((item) => findObjects(item))]
}

function findArrays(value: unknown): Record<string, unknown>[][] {
  if (Array.isArray(value)) {
    const records = value.map(asRecord).filter(Boolean) as Record<
      string,
      unknown
    >[]
    return [
      ...(records.length > 0 ? [records] : []),
      ...value.flatMap((item) => findArrays(item)),
    ]
  }
  const record = asRecord(value)
  if (!record) return []
  return Object.values(record).flatMap((item) => findArrays(item))
}

function pickString(record: Record<string, unknown>, keys: string[]) {
  for (const key of keys) {
    const value = record[key]
    if (typeof value === 'string' && value.trim()) return value.trim()
  }
  return ''
}

function zhipuQuotaTitle(record: Record<string, unknown>) {
  const explicit = pickString(record, ['name', 'title', 'label', 'quotaName'])
  if (explicit) return explicit
  const type = String(record.type ?? '')
  const unit = toFiniteNumber(record.unit)
  const number = toFiniteNumber(record.number)
  if (type === 'TIME_LIMIT' && unit === 5) return 'Every 5 Hours Usage Quota'
  if (unit === 3 || number === 5) return 'Weekly Usage Quota'
  if (unit === 6 || type.includes('MCP')) return 'MCP Monthly Quota'
  return 'Quota Limit'
}

function pickNumber(record: Record<string, unknown>, keys: string[]) {
  for (const key of keys) {
    const value = toFiniteNumber(record[key])
    if (value !== null) return value
  }
  return null
}

function buildQuotaCards(response: ZhipuUsageResponse | null): QuotaCardData[] {
  const quotaPayload = unwrapEndpoint(response, 'quota_limit')
  const candidates = findObjects(quotaPayload)
    .map((record) => {
      const percent =
        pickNumber(record, [
          'usedPercent',
          'usePercent',
          'usagePercent',
          'percent',
          'percentage',
          'rate',
          'ratio',
        ]) ?? null
      if (percent === null) return null
      return {
        title: zhipuQuotaTitle(record),
        percent: percent <= 1 ? percent * 100 : percent,
        resetTime:
          pickString(record, [
            'resetTime',
            'reset_time',
            'nextResetTime',
            'expireTime',
            'endTime',
          ]) || String(record.nextResetTime ?? ''),
      }
    })
    .filter(Boolean) as QuotaCardData[]

  if (candidates.length > 0) return candidates.slice(0, 3)

  const summary = response?.data?.summary
  const total = toFiniteNumber(summary?.total_quota)
  const used = toFiniteNumber(summary?.used_quota)
  const percent = total && used !== null ? (used / total) * 100 : 0
  return [
    { title: 'Every 5 Hours Usage Quota', percent: 0 },
    { title: 'Weekly Usage Quota', percent },
    { title: 'MCP Monthly Quota', percent: 0 },
  ]
}

function getTimeKey(record: Record<string, unknown>) {
  return [
    'time',
    'date',
    'datetime',
    'statTime',
    'startTime',
    'createdTime',
    'timestamp',
    'day',
  ].find((key) => record[key] !== undefined)
}

function buildUsageChartData(payload: unknown): UsageChartData {
  const direct = buildBigModelArrayChartData(payload)
  if (direct.points.length > 0 && direct.series.length > 0) {
    return direct
  }

  const arrays = findArrays(payload)
  const best = arrays
    .filter((items) => items.some((item) => getTimeKey(item)))
    .sort((a, b) => b.length - a.length)[0]

  if (!best) return { points: [], series: [] }

  const timeKey = getTimeKey(best.find((item) => getTimeKey(item)) ?? {}) ?? ''
  const numericKeys = Array.from(
    new Set(
      best.flatMap((item) =>
        Object.keys(item).filter((key) => {
          if (key === timeKey || /id$/i.test(key)) return false
          return toFiniteNumber(item[key]) !== null
        })
      )
    )
  )
    .filter((key) =>
      /(token|usage|use|consume|cost|count|total|amount|value)/i.test(key)
    )
    .slice(0, 4)

  const series = numericKeys.map((key, index) => {
    const total = best.reduce((sum, item) => {
      return sum + (toFiniteNumber(item[key]) ?? 0)
    }, 0)
    return {
      key: `series${index}`,
      label: key,
      color: chartColors[index] ?? chartColors[0],
      total,
    }
  })

  const points = best.map((item) => {
    const point: Record<string, string | number> = {
      time: formatDateLabel(item[timeKey]),
    }
    series.forEach((itemSeries, index) => {
      point[itemSeries.key] = toFiniteNumber(item[numericKeys[index]]) ?? 0
    })
    return point
  })

  return { points, series }
}

function buildBigModelArrayChartData(payload: unknown): UsageChartData {
  const root = asRecord(payload)
  const data = asRecord(root?.data) ?? root
  const xTime = Array.isArray(data?.x_time) ? data.x_time : null
  if (!data || !xTime || xTime.length === 0) {
    return { points: [], series: [] }
  }

  const series: UsageSeries[] = []
  const addSeries = (label: string, values: unknown) => {
    if (!Array.isArray(values) || values.length === 0) return
    const index = series.length
    const key = `series${index}`
    const total = values.reduce(
      (sum, value) => sum + (toFiniteNumber(value) ?? 0),
      0
    )
    series.push({
      key,
      label,
      color: chartColors[index] ?? chartColors[0],
      total,
    })
  }

  addSeries('Token Usage', data.tokensUsage)

  const modelDataList = Array.isArray(data.modelDataList)
    ? data.modelDataList
    : []
  for (const item of modelDataList) {
    const record = asRecord(item)
    if (!record) continue
    addSeries(
      pickString(record, ['modelName', 'modelCode', 'name']) || 'Model Usage',
      record.tokensUsage
    )
  }

  if (series.length === 0) {
    addSeries('Lite Decode Speed', data.liteDecodeSpeed)
    addSeries('Max&Pro Decode Speed', data.proMaxDecodeSpeed)
    addSeries('Lite Success Rate', data.liteSuccessRate)
    addSeries('Max&Pro Success Rate', data.proMaxSuccessRate)
  }

  const points = xTime.map((time, pointIndex) => {
    const point: Record<string, string | number> = {
      time: formatDateLabel(time),
    }
    series.forEach((itemSeries) => {
      const modelRecord = asRecord(
        modelDataList.find((item) => {
          const record = asRecord(item)
          return (
            record &&
            (pickString(record, ['modelName', 'modelCode', 'name']) ||
              'Model Usage') === itemSeries.label
          )
        })
      )
      const sourceValues =
        itemSeries.label === 'Token Usage'
          ? data.tokensUsage
          : modelRecord?.tokensUsage
      if (Array.isArray(sourceValues)) {
        point[itemSeries.key] = toFiniteNumber(sourceValues[pointIndex]) ?? 0
        return
      }
      const performanceMap: Record<string, unknown> = {
        'Lite Decode Speed': data.liteDecodeSpeed,
        'Max&Pro Decode Speed': data.proMaxDecodeSpeed,
        'Lite Success Rate': data.liteSuccessRate,
        'Max&Pro Success Rate': data.proMaxSuccessRate,
      }
      const performanceValues = performanceMap[itemSeries.label]
      point[itemSeries.key] = Array.isArray(performanceValues)
        ? (toFiniteNumber(performanceValues[pointIndex]) ?? 0)
        : 0
    })
    return point
  })

  return { points, series }
}

export function ZhipuUsageDialog({
  open,
  onOpenChange,
  channelName,
  channelId,
  response,
  onRefresh,
  isRefreshing,
}: ZhipuUsageDialogProps) {
  const { t } = useTranslation()
  const [activeUsageTab, setActiveUsageTab] = useState<'model' | 'tool'>(
    'model'
  )
  const [activeRange, setActiveRange] = useState<ZhipuUsageRange>('7d')
  const [showRawJson, setShowRawJson] = useState(false)

  const quotaCards = useMemo(() => buildQuotaCards(response), [response])
  const chartData = useMemo(
    () =>
      buildUsageChartData(
        unwrapEndpoint(
          response,
          activeUsageTab === 'model' ? 'model_usage' : 'model_performance_day'
        )
      ),
    [activeUsageTab, response]
  )
  const chartConfig = useMemo<ChartConfig>(() => {
    return Object.fromEntries(
      chartData.series.map((item) => [
        item.key,
        { label: item.label, color: item.color },
      ])
    )
  }, [chartData.series])
  const rangeLabel = useMemo(() => {
    const { start, end } = getUsageRangeDates(activeRange)
    return `${formatRangeDate(start)} ~ ${formatRangeDate(end)}`
  }, [activeRange])

  const rawJsonText = useMemo(() => {
    if (!response) return ''
    try {
      return JSON.stringify(response, null, 2)
    } catch {
      return String(response)
    }
  }, [response])

  const errorMessage =
    response?.success === false
      ? response.message?.trim() || t('Failed to fetch usage')
      : ''

  return (
    <Dialog
      open={open}
      onOpenChange={(nextOpen) => {
        if (!nextOpen) setShowRawJson(false)
        onOpenChange(nextOpen)
      }}
      title={t('Zhipu Account & Usage')}
      description={`${channelName} (#${channelId})`}
      contentClassName='sm:max-w-[1180px]'
      contentHeight='auto'
      bodyClassName='flex max-h-[78vh] flex-col gap-4 overflow-y-auto'
      footer={
        <Button
          type='button'
          variant='outline'
          onClick={() => onOpenChange(false)}
        >
          {t('Close')}
        </Button>
      }
    >
      {errorMessage && (
        <div className='rounded-lg border border-red-200 bg-red-50 px-4 py-3 text-sm text-red-700 dark:border-red-800 dark:bg-red-950/30 dark:text-red-400'>
          {errorMessage}
        </div>
      )}

      <div className='grid gap-3 md:grid-cols-3'>
        {quotaCards.map((item) => (
          <Card key={item.title} size='sm' className='gap-0'>
            <CardHeader className='pb-3'>
              <CardTitle className='flex items-center gap-1.5 text-sm'>
                {t(item.title)}
                <Info className='text-muted-foreground size-3.5' />
              </CardTitle>
            </CardHeader>
            <CardContent className='space-y-3'>
              <div className='flex items-end gap-1.5'>
                <span className='text-2xl font-semibold'>
                  {Math.round(item.percent)}
                </span>
                <span className='pb-1 text-sm font-medium'>%</span>
                <span className='text-muted-foreground pb-1 text-xs'>
                  {t('used')}
                </span>
              </div>
              <div className='bg-muted h-1.5 overflow-hidden rounded-full'>
                <div
                  className='h-full rounded-full bg-[#2f6bff]'
                  style={{
                    width: `${Math.max(0, Math.min(100, item.percent))}%`,
                  }}
                />
              </div>
              <div className='text-muted-foreground min-h-4 text-xs'>
                {item.resetTime
                  ? `${t('Reset Time:')} ${formatResetTime(item.resetTime)}`
                  : null}
              </div>
            </CardContent>
          </Card>
        ))}
      </div>

      <Card size='sm' className='gap-0'>
        <CardHeader className='flex-row items-center justify-between gap-3 pb-3'>
          <CardTitle>{t('Usage Details')}</CardTitle>
          <CardAction className='flex items-center gap-2'>
            <div className='bg-muted/60 inline-flex h-8 rounded-lg border p-0.5'>
              {usageRangeOptions.map((item) => (
                <button
                  key={item.value}
                  type='button'
                  onClick={() => {
                    if (item.value === activeRange) return
                    setActiveRange(item.value)
                    void onRefresh?.(item.value)
                  }}
                  className={cn(
                    'rounded-md px-3 text-xs font-medium transition-colors',
                    activeRange === item.value
                      ? 'bg-background text-foreground shadow-sm'
                      : 'text-muted-foreground hover:text-foreground'
                  )}
                >
                  {t(item.label)}
                </button>
              ))}
            </div>
            <div className='bg-muted/60 text-muted-foreground inline-flex h-8 items-center gap-1.5 rounded-lg border px-3 text-xs'>
              <Calendar className='size-3.5' />
              <span>{rangeLabel}</span>
            </div>
          </CardAction>
        </CardHeader>
        <CardContent className='space-y-4'>
          <div className='flex flex-wrap items-center justify-between gap-3'>
            <div className='bg-muted/60 inline-flex h-8 rounded-lg border p-0.5'>
              {[
                { value: 'model', label: 'Model Usage' },
                { value: 'tool', label: 'Tool Usage' },
              ].map((item) => (
                <button
                  key={item.value}
                  type='button'
                  onClick={() =>
                    setActiveUsageTab(item.value as 'model' | 'tool')
                  }
                  className={cn(
                    'rounded-md px-3 text-xs font-medium transition-colors',
                    activeUsageTab === item.value
                      ? 'bg-background text-foreground shadow-sm'
                      : 'text-muted-foreground hover:text-foreground'
                  )}
                >
                  {t(item.label)}
                </button>
              ))}
            </div>
            {onRefresh ? (
              <Button
                type='button'
                variant='outline'
                size='sm'
                onClick={() => onRefresh(activeRange)}
                disabled={Boolean(isRefreshing)}
              >
                <RefreshCw data-icon='inline-start' />
                {t('Refresh')}
              </Button>
            ) : null}
          </div>

          <div className='flex flex-wrap gap-6'>
            {chartData.series.map((item) => (
              <div key={item.key} className='min-w-28'>
                <div className='text-muted-foreground flex items-center gap-1.5 text-xs'>
                  <span
                    className='size-1.5 rounded-full'
                    style={{ backgroundColor: item.color }}
                  />
                  {t(item.label)}
                </div>
                <div className='text-2xl font-semibold'>
                  {formatCompact(item.total)}
                </div>
              </div>
            ))}
          </div>

          <div className='bg-muted/30 h-[320px] rounded-md p-4'>
            {chartData.points.length > 0 && chartData.series.length > 0 ? (
              <ChartContainer
                config={chartConfig}
                className='aspect-auto h-full w-full'
              >
                <LineChart
                  data={chartData.points}
                  margin={{ top: 12, right: 16, left: 0, bottom: 4 }}
                >
                  <CartesianGrid vertical={false} />
                  <XAxis
                    dataKey='time'
                    tickLine={false}
                    axisLine={false}
                    minTickGap={36}
                    tickMargin={10}
                  />
                  <YAxis
                    tickLine={false}
                    axisLine={false}
                    tickFormatter={formatCompact}
                    width={42}
                  />
                  <ChartTooltip
                    cursor={true}
                    content={
                      <ChartTooltipContent
                        indicator='dot'
                        labelFormatter={(value) => String(value)}
                        formatter={(value, name) => (
                          <>
                            <span>{t(String(name))}</span>
                            <span className='ml-auto font-mono font-medium'>
                              {formatCompact(value)}
                            </span>
                          </>
                        )}
                      />
                    }
                  />
                  {chartData.series.map((item) => (
                    <Line
                      key={item.key}
                      dataKey={item.key}
                      type='monotone'
                      stroke={`var(--color-${item.key})`}
                      strokeWidth={2}
                      dot={false}
                      activeDot={{ r: 3 }}
                    />
                  ))}
                </LineChart>
              </ChartContainer>
            ) : (
              <div className='text-muted-foreground flex h-full items-center justify-center text-sm'>
                {t('No usage data')}
              </div>
            )}
          </div>
        </CardContent>
      </Card>

      <div className='flex justify-end'>
        <Button
          type='button'
          variant='ghost'
          size='sm'
          onClick={() => setShowRawJson((value) => !value)}
        >
          {showRawJson ? t('Hide Raw JSON') : t('Show Raw JSON')}
        </Button>
      </div>

      {showRawJson ? (
        <ScrollArea className='border-border bg-muted/40 max-h-[320px] rounded-md border'>
          <pre className='p-4 text-xs whitespace-pre-wrap'>{rawJsonText}</pre>
        </ScrollArea>
      ) : null}
    </Dialog>
  )
}
