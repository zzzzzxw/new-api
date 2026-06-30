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
  DatabaseBackupIcon,
  DownloadIcon,
  FileJsonIcon,
  RotateCcwIcon,
  UploadIcon,
} from 'lucide-react'
import { useRef, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert'
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from '@/components/ui/alert-dialog'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Separator } from '@/components/ui/separator'

import { exportSystemBackup, importSystemBackup } from '../api'
import { SettingsSection } from '../components/settings-section'
import type { SystemBackupImportSummary } from '../types'

type BackupRestoreSectionProps = {
  importEndpoint?: string
  onImportSuccess?: (summary: SystemBackupImportSummary) => void
  compact?: boolean
  showExport?: boolean
}

function filenameFromDisposition(header: string | null): string {
  if (!header) return ''
  const match = /filename="?([^";]+)"?/i.exec(header)
  return match?.[1] ?? ''
}

function saveBlob(blob: Blob, filename: string) {
  const url = URL.createObjectURL(blob)
  const anchor = document.createElement('a')
  anchor.href = url
  anchor.download = filename
  document.body.appendChild(anchor)
  anchor.click()
  anchor.remove()
  URL.revokeObjectURL(url)
}

export function BackupRestoreSection(props: BackupRestoreSectionProps) {
  const { t } = useTranslation()
  const fileInputRef = useRef<HTMLInputElement | null>(null)
  const [selectedFile, setSelectedFile] = useState<File | null>(null)
  const [exporting, setExporting] = useState(false)
  const [importing, setImporting] = useState(false)
  const [confirmOpen, setConfirmOpen] = useState(false)
  const showExport = props.showExport ?? true

  const handleExport = async () => {
    setExporting(true)
    try {
      const response = await exportSystemBackup()
      const filename =
        filenameFromDisposition(response.headers['content-disposition']) ||
        `new-api-backup-${Date.now()}.json`
      saveBlob(response.data, filename)
      toast.success(t('Backup exported successfully'))
    } catch {
      toast.error(t('Failed to export backup'))
    } finally {
      setExporting(false)
    }
  }

  const handleImport = async () => {
    if (!selectedFile) {
      toast.error(t('Select a backup file first'))
      return
    }

    setImporting(true)
    try {
      const response = await importSystemBackup(
        selectedFile,
        props.importEndpoint
      )
      if (!response.success || !response.data) {
        toast.error(response.message || t('Failed to import backup'))
        return
      }
      toast.success(
        t('Backup imported: {{rows}} rows across {{tables}} tables', {
          rows: response.data.imported_rows,
          tables: response.data.imported_tables,
        })
      )
      setSelectedFile(null)
      if (fileInputRef.current) fileInputRef.current.value = ''
      props.onImportSuccess?.(response.data)
    } catch {
      toast.error(t('Failed to import backup'))
    } finally {
      setImporting(false)
      setConfirmOpen(false)
    }
  }

  const content = (
    <div className='space-y-5'>
      <Alert className='border-amber-200 bg-amber-50 dark:border-amber-900/60 dark:bg-amber-950/40'>
        <DatabaseBackupIcon className='size-4 text-amber-500' />
        <AlertTitle>{t('Backup contains sensitive data')}</AlertTitle>
        <AlertDescription>
          {t(
            'The exported file includes users, tokens, channels, keys, billing records, and system settings. Store it securely.'
          )}
        </AlertDescription>
      </Alert>

      <div className={showExport ? 'grid gap-4 md:grid-cols-2' : 'grid gap-4'}>
        {showExport && (
          <div className='rounded-lg border p-4'>
            <div className='flex items-start gap-3'>
              <div className='bg-primary/10 text-primary flex size-9 shrink-0 items-center justify-center rounded-md'>
                <DownloadIcon className='size-4' />
              </div>
              <div className='min-w-0 flex-1 space-y-3'>
                <div>
                  <h3 className='font-medium'>{t('Export backup')}</h3>
                  <p className='text-muted-foreground mt-1 text-sm'>
                    {t('Download a JSON backup of the current main database.')}
                  </p>
                </div>
                <Button onClick={handleExport} disabled={exporting}>
                  {exporting ? (
                    t('Exporting...')
                  ) : (
                    <>
                      <DownloadIcon className='me-2 size-4' />
                      {t('Export backup')}
                    </>
                  )}
                </Button>
              </div>
            </div>
          </div>
        )}

        <div className='rounded-lg border p-4'>
          <div className='flex items-start gap-3'>
            <div className='bg-destructive/10 text-destructive flex size-9 shrink-0 items-center justify-center rounded-md'>
              <UploadIcon className='size-4' />
            </div>
            <div className='min-w-0 flex-1 space-y-3'>
              <div>
                <h3 className='font-medium'>{t('Import backup')}</h3>
                <p className='text-muted-foreground mt-1 text-sm'>
                  {t(
                    'Restore a backup file and replace the current database data.'
                  )}
                </p>
              </div>
              <div className='space-y-2'>
                <Input
                  ref={fileInputRef}
                  type='file'
                  accept='application/json,.json'
                  disabled={importing}
                  onChange={(event) => {
                    setSelectedFile(event.target.files?.[0] ?? null)
                  }}
                />
                {selectedFile && (
                  <div className='text-muted-foreground flex min-w-0 items-center gap-2 text-xs'>
                    <FileJsonIcon className='size-3.5 shrink-0' />
                    <span className='truncate'>{selectedFile.name}</span>
                  </div>
                )}
              </div>
              <Button
                variant='destructive'
                disabled={!selectedFile || importing}
                onClick={() => setConfirmOpen(true)}
              >
                {importing ? (
                  t('Importing...')
                ) : (
                  <>
                    <UploadIcon className='me-2 size-4' />
                    {t('Import backup')}
                  </>
                )}
              </Button>
            </div>
          </div>
        </div>
      </div>

      {!props.compact && (
        <>
          <Separator />
          <Button
            type='button'
            variant='outline'
            onClick={() => window.location.reload()}
          >
            <RotateCcwIcon className='me-2 size-4' />
            {t('Reload page')}
          </Button>
        </>
      )}

      <AlertDialog open={confirmOpen} onOpenChange={setConfirmOpen}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>{t('Import backup?')}</AlertDialogTitle>
            <AlertDialogDescription>
              {t(
                'This will replace current database records with the selected backup. This action cannot be undone.'
              )}
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel disabled={importing}>
              {t('Cancel')}
            </AlertDialogCancel>
            <AlertDialogAction
              disabled={importing}
              onClick={(event) => {
                event.preventDefault()
                handleImport()
              }}
            >
              {importing ? t('Importing...') : t('Import backup')}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </div>
  )

  if (props.compact) return content

  return (
    <SettingsSection title={t('Backup & Restore')}>{content}</SettingsSection>
  )
}
