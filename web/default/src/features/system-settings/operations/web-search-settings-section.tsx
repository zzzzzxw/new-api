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
import * as z from 'zod'
import { useMemo } from 'react'
import { useForm } from 'react-hook-form'
import { zodResolver } from '@hookform/resolvers/zod'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
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
import { SettingsForm } from '../components/settings-form-layout'
import { SettingsPageFormActions } from '../components/settings-page-context'
import { SettingsSection } from '../components/settings-section'
import { useResetForm } from '../hooks/use-reset-form'
import { useUpdateOption } from '../hooks/use-update-option'

const PROVIDER_DUCKDUCKGO = 'duckduckgo'
const PROVIDER_TAVILY = 'tavily'

// NOTE: react-hook-form treats dots in field names as nested paths
// (isKey: /^\w*$/), so the form uses flat camelCase names and maps
// them to the real option keys only when saving.
const webSearchSchema = z.object({
  provider: z.string(),
  tavilyApiKey: z.string(),
})

type WebSearchFormValues = z.infer<typeof webSearchSchema>

type WebSearchSettingsSectionProps = {
  defaultValues: {
    'web_search_setting.provider': string
    'web_search_setting.tavily_api_key': string
  }
}

export function WebSearchSettingsSection({
  defaultValues,
}: WebSearchSettingsSectionProps) {
  const { t } = useTranslation()
  const updateOption = useUpdateOption()

  const initialValues = useMemo<WebSearchFormValues>(
    () => ({
      provider: defaultValues['web_search_setting.provider'],
      tavilyApiKey: defaultValues['web_search_setting.tavily_api_key'],
    }),
    [defaultValues]
  )

  const form = useForm<WebSearchFormValues>({
    resolver: zodResolver(webSearchSchema),
    defaultValues: initialValues,
  })

  useResetForm(form, initialValues)

  const provider = form.watch('provider')

  const onSubmit = async (values: WebSearchFormValues) => {
    const updates: Array<{ key: string; value: string }> = []

    if (values.provider !== initialValues.provider) {
      updates.push({
        key: 'web_search_setting.provider',
        value: values.provider,
      })
    }

    const apiKey = values.tavilyApiKey.trim()
    if (apiKey !== initialValues.tavilyApiKey) {
      updates.push({
        key: 'web_search_setting.tavily_api_key',
        value: apiKey,
      })
    }

    for (const update of updates) {
      await updateOption.mutateAsync(update)
    }

    if (updates.length === 0) {
      toast.info(t('No changes to save'))
    }
  }

  return (
    <SettingsSection title={t('Web Search')}>
      <Form {...form}>
        <SettingsForm onSubmit={form.handleSubmit(onSubmit)} autoComplete='off'>
          <SettingsPageFormActions
            onSave={form.handleSubmit(onSubmit)}
            isSaving={updateOption.isPending}
            saveLabel='Save web search settings'
          />
          <FormField
            control={form.control}
            name='provider'
            render={({ field }) => (
              <FormItem>
                <FormLabel>{t('Search Provider')}</FormLabel>
                <Select
                  items={[
                    {
                      value: PROVIDER_DUCKDUCKGO,
                      label: t('DuckDuckGo (free, built-in)'),
                    },
                    { value: PROVIDER_TAVILY, label: 'Tavily' },
                  ]}
                  value={field.value}
                  onValueChange={field.onChange}
                >
                  <FormControl>
                    <SelectTrigger>
                      <SelectValue />
                    </SelectTrigger>
                  </FormControl>
                  <SelectContent alignItemWithTrigger={false}>
                    <SelectGroup>
                      <SelectItem value={PROVIDER_DUCKDUCKGO}>
                        {t('DuckDuckGo (free, built-in)')}
                      </SelectItem>
                      <SelectItem value={PROVIDER_TAVILY}>Tavily</SelectItem>
                    </SelectGroup>
                  </SelectContent>
                </Select>
                <FormDescription>
                  {t(
                    'DuckDuckGo only answers keyword-style queries. For general web search, use Tavily. If the selected provider fails, DuckDuckGo is used as fallback.'
                  )}
                </FormDescription>
                <FormMessage />
              </FormItem>
            )}
          />
          {provider === PROVIDER_TAVILY && (
            <FormField
              control={form.control}
              name='tavilyApiKey'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t('Tavily API Key')}</FormLabel>
                  <FormControl>
                    <Input
                      {...field}
                      type='password'
                      placeholder='tvly-...'
                      autoComplete='new-password'
                    />
                  </FormControl>
                  <FormDescription>
                    {t(
                      'Get a free API key at tavily.com (1,000 requests/month on the free plan).'
                    )}
                  </FormDescription>
                  <FormMessage />
                </FormItem>
              )}
            />
          )}
        </SettingsForm>
      </Form>
    </SettingsSection>
  )
}
