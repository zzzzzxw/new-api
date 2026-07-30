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
import { CalendarClock, RefreshCw, UserRound } from 'lucide-react'
import { useMemo, useState } from 'react'
import { useTranslation } from 'react-i18next'

import { Dialog } from '@/components/dialog'
import { StatusBadge } from '@/components/status-badge'
import { Button } from '@/components/ui/button'
import {
  Card,
  CardAction,
  CardContent,
  CardHeader,
  CardTitle,
} from '@/components/ui/card'
import { Progress } from '@/components/ui/progress'
import { ScrollArea } from '@/components/ui/scroll-area'

import type { GrokUsageResponse, GrokUsageSummary } from '../../api'

type GrokUsageDialogProps = {
  open: boolean
  onOpenChange: (open: boolean) => void
  channelName: string
  channelId: number
  response: GrokUsageResponse | null
  onRefresh?: () => void | Promise<void>
  isRefreshing?: boolean
}

function formatDateTime(value: string | undefined): string {
  if (!value) {
    return '-'
  }
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) {
    return value
  }
  return new Intl.DateTimeFormat(undefined, {
    dateStyle: 'medium',
    timeStyle: 'short',
  }).format(date)
}

function formatCents(value: number | undefined): string {
  if (!Number.isFinite(value)) {
    return '-'
  }
  return new Intl.NumberFormat(undefined, {
    style: 'currency',
    currency: 'USD',
    minimumFractionDigits: 2,
    maximumFractionDigits: 2,
  }).format(Math.abs(Number(value)) / 100)
}

function accountName(summary: GrokUsageSummary | undefined): string {
  const name = [summary?.first_name, summary?.last_name]
    .filter(Boolean)
    .join(' ')
    .trim()
  return name || summary?.email || '-'
}

function periodLabel(
  periodType: string | undefined,
  t: (key: string) => string
): string {
  const normalized = periodType?.toUpperCase() || ''
  if (normalized.includes('WEEKLY')) {
    return t('Weekly limit')
  }
  if (normalized.includes('MONTHLY')) {
    return t('Monthly limit')
  }
  return t('Usage')
}

function DetailItem(props: { label: string; value: string }) {
  return (
    <div className='bg-background min-w-0 rounded-lg border px-3 py-2.5'>
      <div className='text-muted-foreground text-xs'>{props.label}</div>
      <div className='mt-1 truncate text-sm font-medium' title={props.value}>
        {props.value || '-'}
      </div>
    </div>
  )
}

function BillingCard(props: {
  title: string
  value: string
  description?: string
}) {
  return (
    <Card size='sm' className='gap-0'>
      <CardHeader className='pb-2'>
        <CardTitle className='text-muted-foreground text-xs font-medium'>
          {props.title}
        </CardTitle>
      </CardHeader>
      <CardContent>
        <div className='text-xl font-semibold'>{props.value}</div>
        {props.description ? (
          <div className='text-muted-foreground mt-1 text-xs'>
            {props.description}
          </div>
        ) : null}
      </CardContent>
    </Card>
  )
}

export function GrokUsageDialog(props: GrokUsageDialogProps) {
  const { t } = useTranslation()
  const [showRawJson, setShowRawJson] = useState(false)
  const summary = props.response?.data?.summary
  const usagePercent = Math.max(
    0,
    Math.min(100, Number(summary?.credit_usage_percent) || 0)
  )
  const rawJsonText = useMemo(() => {
    if (!props.response) {
      return ''
    }
    try {
      return JSON.stringify(props.response, null, 2)
    } catch {
      return String(props.response)
    }
  }, [props.response])

  const endpointStatuses = Object.entries(props.response?.data?.endpoints || {})
  const hasPartialWarning = Boolean(
    props.response?.success && props.response.message?.trim()
  )
  const errorMessage =
    props.response?.success === false
      ? props.response.message?.trim() || t('Failed to fetch usage')
      : ''
  let autoTopupValue = '-'
  if (summary?.auto_topup_enabled != null) {
    autoTopupValue = summary.auto_topup_enabled ? t('Enabled') : t('Disabled')
  }

  return (
    <Dialog
      open={props.open}
      onOpenChange={(open) => {
        if (!open) {
          setShowRawJson(false)
        }
        props.onOpenChange(open)
      }}
      title={t('Grok Account & Usage')}
      description={`${props.channelName} (#${props.channelId})`}
      contentClassName='sm:max-w-[920px]'
      contentHeight='auto'
      bodyClassName='flex max-h-[78vh] flex-col gap-4 overflow-y-auto'
      footer={
        <Button
          type='button'
          variant='outline'
          onClick={() => props.onOpenChange(false)}
        >
          {t('Close')}
        </Button>
      }
    >
      {errorMessage ? (
        <div className='rounded-lg border border-red-200 bg-red-50 px-4 py-3 text-sm text-red-700 dark:border-red-800 dark:bg-red-950/30 dark:text-red-400'>
          {errorMessage}
        </div>
      ) : null}
      {hasPartialWarning ? (
        <div className='rounded-lg border border-amber-200 bg-amber-50 px-4 py-3 text-sm text-amber-800 dark:border-amber-800 dark:bg-amber-950/30 dark:text-amber-300'>
          {props.response?.message}
        </div>
      ) : null}

      <Card size='sm' className='bg-muted/30 gap-0 py-0'>
        <CardHeader className='p-4 pb-2'>
          <CardTitle className='flex items-center gap-2 text-sm'>
            <UserRound className='size-4' />
            {t('Grok Account Status')}
          </CardTitle>
          {props.onRefresh ? (
            <CardAction>
              <Button
                type='button'
                variant='outline'
                size='sm'
                onClick={props.onRefresh}
                disabled={Boolean(props.isRefreshing)}
              >
                <RefreshCw
                  className={props.isRefreshing ? 'animate-spin' : undefined}
                />
                {t('Refresh')}
              </Button>
            </CardAction>
          ) : null}
        </CardHeader>
        <CardContent className='space-y-3 p-4 pt-1'>
          <div className='flex flex-wrap items-center gap-2'>
            <StatusBadge
              label={summary?.subscription_tier || t('Unknown')}
              variant={summary?.subscription_tier ? 'success' : 'neutral'}
              copyable={false}
            />
            {summary?.principal_type ? (
              <StatusBadge
                label={summary.principal_type}
                variant='blue'
                copyable={false}
              />
            ) : null}
            {endpointStatuses.map(([name, endpoint]) => (
              <StatusBadge
                key={name}
                label={`${name}: HTTP ${endpoint.status || '-'}`}
                variant={
                  endpoint.status &&
                  endpoint.status >= 200 &&
                  endpoint.status < 300
                    ? 'success'
                    : 'neutral'
                }
                copyable={false}
              />
            ))}
          </div>
          <div className='grid gap-2 sm:grid-cols-2 lg:grid-cols-3'>
            <DetailItem label={t('Account')} value={accountName(summary)} />
            <DetailItem label={t('Email')} value={summary?.email || '-'} />
            <DetailItem label={t('User ID')} value={summary?.user_id || '-'} />
            <DetailItem
              label={t('Team')}
              value={summary?.team_name || summary?.team_id || '-'}
            />
            <DetailItem
              label={t('Organization')}
              value={
                summary?.organization_name || summary?.organization_id || '-'
              }
            />
            <DetailItem
              label={t('Credential expires')}
              value={formatDateTime(summary?.credential_expires_at)}
            />
          </div>
        </CardContent>
      </Card>

      <Card size='sm' className='gap-0'>
        <CardHeader className='pb-3'>
          <CardTitle className='flex items-center gap-2'>
            <CalendarClock className='size-4' />
            {t('Usage Details')}
          </CardTitle>
        </CardHeader>
        <CardContent className='space-y-3'>
          <div className='flex items-end justify-between gap-3'>
            <div>
              <div className='text-muted-foreground text-xs'>
                {periodLabel(summary?.usage_period_type, t)}
              </div>
              <div className='mt-1 text-3xl font-semibold'>
                {Math.floor(usagePercent)}%
              </div>
            </div>
            <div className='text-muted-foreground text-right text-xs'>
              <div>
                {t('Usage period')}:{' '}
                {formatDateTime(summary?.usage_period_start)}
              </div>
              <div>
                {t('Next reset')}: {formatDateTime(summary?.usage_period_end)}
              </div>
            </div>
          </div>
          <Progress value={usagePercent} />
        </CardContent>
      </Card>

      <div className='grid gap-3 sm:grid-cols-2 lg:grid-cols-4'>
        <BillingCard
          title={t('Included usage')}
          value={formatCents(summary?.used_cents)}
          description={
            summary?.monthly_limit_cents == null
              ? undefined
              : `${t('Limit')}: ${formatCents(summary.monthly_limit_cents)}`
          }
        />
        <BillingCard
          title={t('Prepaid credits')}
          value={formatCents(summary?.prepaid_balance_cents)}
          description={
            summary?.is_unified_billing_user ? t('Unified billing') : undefined
          }
        />
        <BillingCard
          title={t('Pay-as-you-go')}
          value={formatCents(summary?.on_demand_used_cents)}
          description={
            summary?.on_demand_cap_cents == null
              ? undefined
              : `${t('Limit')}: ${formatCents(summary.on_demand_cap_cents)}`
          }
        />
        <BillingCard
          title={t('Auto top-up')}
          value={autoTopupValue}
          description={
            summary?.auto_topup_amount_cents == null
              ? undefined
              : `${t('Top-up amount')}: ${formatCents(
                  summary.auto_topup_amount_cents
                )}`
          }
        />
      </div>

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
        <Card size='sm' className='gap-0'>
          <CardHeader className='pb-2'>
            <CardTitle>{t('Raw JSON')}</CardTitle>
          </CardHeader>
          <CardContent>
            <ScrollArea className='bg-muted/30 h-64 rounded-md border'>
              <pre className='p-4 text-xs whitespace-pre-wrap'>
                {rawJsonText}
              </pre>
            </ScrollArea>
          </CardContent>
        </Card>
      ) : null}
    </Dialog>
  )
}
