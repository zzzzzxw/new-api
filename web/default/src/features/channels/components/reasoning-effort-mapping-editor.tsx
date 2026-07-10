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
import { useCallback, useEffect, useId, useMemo, useRef, useState } from 'react'
import { Code, Plus, Table, Trash2 } from 'lucide-react'
import { useTranslation } from 'react-i18next'

import { Alert, AlertDescription } from '@/components/ui/alert'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs'
import { Textarea } from '@/components/ui/textarea'
import { cn } from '@/lib/utils'

type ReasoningEffortMappingEditorProps = {
  value: string
  onChange: (value: string) => void
  disabled?: boolean
  modelOptions?: string[]
}

type ReasoningEffortMappingRow = {
  id: string
  model: string
  originalReasoningEffort: string
  replacementReasoningEffort: string
}

type ReasoningEffortMappingValue = {
  model: string
  original_reasoning_effort: string
  replacement_reasoning_effort: string
}

const DUPLICATE_MAPPING_SENTINEL =
  '[{"duplicate_reasoning_effort_mapping":true}]'
const REASONING_EFFORT_OPTIONS = [
  'minimal',
  'low',
  'medium',
  'high',
  'xhigh',
  'max',
  'none',
]

function getDuplicateMappings(rows: ReasoningEffortMappingRow[]): string[] {
  const seen = new Set<string>()
  const duplicates = new Set<string>()

  for (const row of rows) {
    const model = row.model.trim()
    const effort = row.originalReasoningEffort.trim().toLowerCase()
    if (!model || !effort) continue
    const key = `${model}\u0000${effort}`
    if (seen.has(key)) {
      duplicates.add(`${model} / ${effort}`)
    } else {
      seen.add(key)
    }
  }

  return [...duplicates]
}

function isMappingValue(value: unknown): value is ReasoningEffortMappingValue {
  if (!value || typeof value !== 'object' || Array.isArray(value)) return false
  const item = value as Record<string, unknown>
  return (
    typeof item.model === 'string' &&
    typeof item.original_reasoning_effort === 'string' &&
    typeof item.replacement_reasoning_effort === 'string'
  )
}

export function ReasoningEffortMappingEditor(
  props: ReasoningEffortMappingEditorProps
) {
  const { t } = useTranslation()
  const modelListId = useId()
  const effortListId = useId()
  const [mode, setMode] = useState<'visual' | 'json'>('visual')
  const [rows, setRows] = useState<ReasoningEffortMappingRow[]>([])
  const [jsonValue, setJsonValue] = useState(props.value)
  const [jsonError, setJsonError] = useState<string | null>(null)
  const nextRowIdRef = useRef(0)
  const duplicateMappings = useMemo(() => getDuplicateMappings(rows), [rows])

  const createRowId = useCallback(() => {
    nextRowIdRef.current += 1
    return `reasoning-effort-mapping-${nextRowIdRef.current}`
  }, [])

  const parseJsonToRows = useCallback(
    (json: string): boolean => {
      try {
        if (!json.trim()) {
          setRows([])
          setJsonError(null)
          return true
        }
        const parsed: unknown = JSON.parse(json)
        if (!Array.isArray(parsed) || !parsed.every(isMappingValue)) {
          setJsonError(t('Reasoning effort mapping must be a valid JSON array'))
          return false
        }

        const parsedRows = parsed.map((item) => ({
          id: createRowId(),
          model: item.model,
          originalReasoningEffort: item.original_reasoning_effort,
          replacementReasoningEffort: item.replacement_reasoning_effort,
        }))
        if (
          parsedRows.some(
            (row) =>
              !row.model.trim() ||
              !row.originalReasoningEffort.trim() ||
              !row.replacementReasoningEffort.trim()
          )
        ) {
          setJsonError(t('All reasoning effort mapping fields are required'))
          return false
        }
        if (getDuplicateMappings(parsedRows).length > 0) {
          setJsonError(
            t(
              'Duplicate model and original reasoning effort mappings are not allowed'
            )
          )
          return false
        }

        setRows(parsedRows)
        setJsonError(null)
        return true
      } catch {
        setJsonError(t('Reasoning effort mapping must be valid JSON'))
        return false
      }
    },
    [createRowId, t]
  )

  useEffect(() => {
    // eslint-disable-next-line react-hooks/set-state-in-effect
    setJsonValue(props.value)
    parseJsonToRows(props.value)
  }, [parseJsonToRows, props.value])

  const convertRowsToJson = (
    updatedRows: ReasoningEffortMappingRow[]
  ): string => {
    if (updatedRows.length === 0) return ''
    const value = updatedRows
      .filter(
        (row) =>
          row.model.trim() ||
          row.originalReasoningEffort.trim() ||
          row.replacementReasoningEffort.trim()
      )
      .map((row) => ({
        model: row.model.trim(),
        original_reasoning_effort: row.originalReasoningEffort
          .trim()
          .toLowerCase(),
        replacement_reasoning_effort: row.replacementReasoningEffort
          .trim()
          .toLowerCase(),
      }))
    return value.length > 0 ? JSON.stringify(value, null, 2) : ''
  }

  const syncRows = (updatedRows: ReasoningEffortMappingRow[]) => {
    setRows(updatedRows)
    const duplicates = getDuplicateMappings(updatedRows)
    if (duplicates.length > 0) {
      setJsonError(
        t(
          'Duplicate model and original reasoning effort mappings are not allowed'
        )
      )
      setJsonValue(DUPLICATE_MAPPING_SENTINEL)
      props.onChange(DUPLICATE_MAPPING_SENTINEL)
      return
    }

    const json = convertRowsToJson(updatedRows)
    setJsonError(null)
    setJsonValue(json)
    props.onChange(json)
  }

  const handleAddRow = () => {
    syncRows([
      ...rows,
      {
        id: createRowId(),
        model: '',
        originalReasoningEffort: '',
        replacementReasoningEffort: '',
      },
    ])
  }

  const handleRowChange = (
    id: string,
    field: 'model' | 'originalReasoningEffort' | 'replacementReasoningEffort',
    value: string
  ) => {
    syncRows(
      rows.map((row) => (row.id === id ? { ...row, [field]: value } : row))
    )
  }

  const handleJsonChange = (value: string) => {
    setJsonValue(value)
    props.onChange(value)
    parseJsonToRows(value)
  }

  const handleFillTemplate = () => {
    const template = JSON.stringify(
      [
        {
          model: 'glm-5.2',
          original_reasoning_effort: 'medium',
          replacement_reasoning_effort: 'high',
        },
      ],
      null,
      2
    )
    setJsonValue(template)
    props.onChange(template)
    parseJsonToRows(template)
  }

  const handleModeChange = (nextMode: string) => {
    if (nextMode !== 'visual' && nextMode !== 'json') return
    if (nextMode === 'json') {
      if (duplicateMappings.length === 0) {
        const json = convertRowsToJson(rows)
        setJsonValue(json)
        props.onChange(json)
      }
      setMode('json')
      return
    }
    parseJsonToRows(jsonValue)
    setMode('visual')
  }

  return (
    <div className='space-y-2'>
      <Tabs value={mode} onValueChange={handleModeChange} className='space-y-2'>
        <div className='flex items-center justify-between gap-3'>
          <TabsList>
            <TabsTrigger value='visual'>
              <Table className='h-4 w-4' aria-hidden='true' />
              {t('Visual')}
            </TabsTrigger>
            <TabsTrigger value='json'>
              <Code className='h-4 w-4' aria-hidden='true' />
              {t('JSON')}
            </TabsTrigger>
          </TabsList>
          <Button
            type='button'
            variant='link'
            size='sm'
            className='h-auto p-0'
            onClick={handleFillTemplate}
            disabled={props.disabled}
          >
            {t('Fill Template')}
          </Button>
        </div>

        {jsonError && (
          <Alert variant='destructive'>
            <AlertDescription>{jsonError}</AlertDescription>
          </Alert>
        )}

        {duplicateMappings.length > 0 && (
          <Alert>
            <AlertDescription>
              {t('Duplicate reasoning effort mapping(s): {{mappings}}', {
                mappings: duplicateMappings.join(', '),
              })}
            </AlertDescription>
          </Alert>
        )}

        <TabsContent value='visual' className='space-y-2'>
          {rows.length > 0 ? (
            <div className='space-y-2'>
              <div className='hidden grid-cols-[1.2fr_1fr_1fr_auto] gap-2 text-sm font-medium sm:grid'>
                <div>{t('Model')}</div>
                <div>{t('Original Reasoning Effort')}</div>
                <div>{t('Replacement Reasoning Effort')}</div>
                <div className='w-10' />
              </div>
              {rows.map((row) => (
                <div
                  key={row.id}
                  className='grid grid-cols-1 gap-2 sm:grid-cols-[1.2fr_1fr_1fr_auto]'
                >
                  <Input
                    value={row.model}
                    onChange={(event) =>
                      handleRowChange(row.id, 'model', event.target.value)
                    }
                    placeholder='glm-5.2'
                    disabled={props.disabled}
                    list={modelListId}
                    aria-label={t('Model')}
                  />
                  <Input
                    value={row.originalReasoningEffort}
                    onChange={(event) =>
                      handleRowChange(
                        row.id,
                        'originalReasoningEffort',
                        event.target.value
                      )
                    }
                    placeholder='medium'
                    disabled={props.disabled}
                    list={effortListId}
                    aria-label={t('Original Reasoning Effort')}
                  />
                  <Input
                    value={row.replacementReasoningEffort}
                    onChange={(event) =>
                      handleRowChange(
                        row.id,
                        'replacementReasoningEffort',
                        event.target.value
                      )
                    }
                    placeholder='high'
                    disabled={props.disabled}
                    list={effortListId}
                    aria-label={t('Replacement Reasoning Effort')}
                  />
                  <Button
                    type='button'
                    variant='ghost'
                    size='icon'
                    onClick={() =>
                      syncRows(rows.filter((item) => item.id !== row.id))
                    }
                    disabled={props.disabled}
                    className='h-10 w-10 justify-self-end'
                    aria-label={t('Delete mapping')}
                  >
                    <Trash2 className='h-4 w-4' aria-hidden='true' />
                  </Button>
                </div>
              ))}
            </div>
          ) : (
            <div className='text-muted-foreground flex h-24 items-center justify-center rounded-md border border-dashed px-4 text-center text-sm'>
              {t(
                'No reasoning effort mappings configured. The default mapping logic will be used.'
              )}
            </div>
          )}
          <Button
            type='button'
            variant='outline'
            size='sm'
            onClick={handleAddRow}
            disabled={props.disabled}
            className='w-full'
          >
            <Plus className='mr-2 h-4 w-4' aria-hidden='true' />
            {t('Add Mapping')}
          </Button>
        </TabsContent>

        <TabsContent value='json'>
          <Textarea
            value={jsonValue}
            onChange={(event) => handleJsonChange(event.target.value)}
            placeholder='[{"model":"glm-5.2","original_reasoning_effort":"medium","replacement_reasoning_effort":"high"}]'
            disabled={props.disabled}
            rows={10}
            className={cn(
              'font-mono text-sm',
              jsonError && 'border-destructive'
            )}
            aria-invalid={Boolean(jsonError)}
          />
        </TabsContent>
      </Tabs>

      {props.modelOptions && props.modelOptions.length > 0 && (
        <datalist id={modelListId}>
          {props.modelOptions.map((model) => (
            <option key={model} value={model} />
          ))}
        </datalist>
      )}
      <datalist id={effortListId}>
        {REASONING_EFFORT_OPTIONS.map((effort) => (
          <option key={effort} value={effort} />
        ))}
      </datalist>
    </div>
  )
}
