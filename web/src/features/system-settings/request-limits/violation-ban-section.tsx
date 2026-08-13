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
import { Plus, Trash2 } from 'lucide-react'
import { useEffect } from 'react'
import { useFieldArray, useForm } from 'react-hook-form'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import * as z from 'zod'

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
import { Switch } from '@/components/ui/switch'
import {
  Tooltip,
  TooltipContent,
  TooltipTrigger,
} from '@/components/ui/tooltip'

import {
  SettingsForm,
  SettingsSwitchContent,
  SettingsSwitchItem,
} from '../components/settings-form-layout'
import { SettingsPageFormActions } from '../components/settings-page-context'
import { SettingsSection } from '../components/settings-section'
import { useUpdateOption } from '../hooks/use-update-option'

const maxRules = 32
const maxKeywordLength = 256
const maxThreshold = 2147483647
const maxWindowHours = 24 * 365

type ViolationBanRule = {
  statusCode: number
  keyword: string
}

type ViolationBanSectionProps = {
  defaultValues: {
    UserViolationBanEnabled: boolean
    UserViolationBanRules: string
    UserViolationBanThreshold: number
    UserViolationBanWindowHours: number
  }
}

const createViolationBanSchema = () =>
  z
    .object({
      UserViolationBanEnabled: z.boolean(),
      UserViolationBanThreshold: z.number().int().min(0).max(maxThreshold),
      UserViolationBanWindowHours: z.number().int().min(0).max(maxWindowHours),
      UserViolationBanRules: z
        .array(
          z.object({
            statusCode: z.number().int().min(100).max(599),
            keyword: z.string().trim().min(1).max(maxKeywordLength),
          })
        )
        .max(maxRules),
    })
    .superRefine((values, context) => {
      if (!values.UserViolationBanEnabled) return

      if (values.UserViolationBanRules.length === 0) {
        context.addIssue({
          code: 'custom',
          message:
            'At least one violation rule is required when automatic banning is enabled',
          path: ['UserViolationBanRules'],
        })
      }
      if (values.UserViolationBanThreshold < 1) {
        context.addIssue({
          code: 'custom',
          message:
            'The ban threshold must be at least 1 when automatic banning is enabled',
          path: ['UserViolationBanThreshold'],
        })
      }
      if (values.UserViolationBanWindowHours < 1) {
        context.addIssue({
          code: 'custom',
          message:
            'The sliding window must be at least 1 hour when automatic banning is enabled',
          path: ['UserViolationBanWindowHours'],
        })
      }
    })

type ViolationBanFormValues = z.output<
  ReturnType<typeof createViolationBanSchema>
>
type ViolationBanFormInput = z.input<
  ReturnType<typeof createViolationBanSchema>
>

function parseViolationBanRules(value: string): ViolationBanRule[] {
  try {
    const parsed: unknown = JSON.parse(value)
    if (!Array.isArray(parsed)) return []

    return parsed.flatMap((rule) => {
      if (!rule || typeof rule !== 'object') return []

      const { status_code: statusCode, keyword } = rule as {
        status_code?: unknown
        keyword?: unknown
      }
      if (typeof statusCode !== 'number' || typeof keyword !== 'string') {
        return []
      }
      return [{ statusCode, keyword }]
    })
  } catch {
    return []
  }
}

function buildFormDefaults(
  defaults: ViolationBanSectionProps['defaultValues']
): ViolationBanFormInput {
  return {
    UserViolationBanEnabled: defaults.UserViolationBanEnabled,
    UserViolationBanRules: parseViolationBanRules(
      defaults.UserViolationBanRules
    ),
    UserViolationBanThreshold: defaults.UserViolationBanThreshold,
    UserViolationBanWindowHours: defaults.UserViolationBanWindowHours,
  }
}

function serializeViolationBanRules(rules: ViolationBanRule[]): string {
  return JSON.stringify(
    rules.map((rule) => ({
      status_code: rule.statusCode,
      keyword: rule.keyword.trim(),
    }))
  )
}

export function ViolationBanSection(props: ViolationBanSectionProps) {
  const { t } = useTranslation()
  const updateOption = useUpdateOption()
  const violationBanSchema = createViolationBanSchema()
  const form = useForm<ViolationBanFormInput, unknown, ViolationBanFormValues>({
    resolver: zodResolver(violationBanSchema),
    mode: 'onChange',
    defaultValues: buildFormDefaults(props.defaultValues),
  })
  const rules = useFieldArray({
    control: form.control,
    name: 'UserViolationBanRules',
  })

  useEffect(() => {
    form.reset(buildFormDefaults(props.defaultValues))
  }, [form, props.defaultValues])

  const onSubmit = async (values: ViolationBanFormValues) => {
    const defaultRules = parseViolationBanRules(
      props.defaultValues.UserViolationBanRules
    )
    const serializedRules = serializeViolationBanRules(
      values.UserViolationBanRules
    )
    const rulesChanged =
      serializedRules !== serializeViolationBanRules(defaultRules)
    const thresholdChanged =
      values.UserViolationBanThreshold !==
      props.defaultValues.UserViolationBanThreshold
    const windowHoursChanged =
      values.UserViolationBanWindowHours !==
      props.defaultValues.UserViolationBanWindowHours
    const enabledChanged =
      values.UserViolationBanEnabled !==
      props.defaultValues.UserViolationBanEnabled

    if (
      !rulesChanged &&
      !thresholdChanged &&
      !windowHoursChanged &&
      !enabledChanged
    ) {
      toast.info(t('No changes to save'))
      return
    }

    const configurationUpdates = []
    if (rulesChanged) {
      configurationUpdates.push({
        key: 'UserViolationBanRules',
        value: serializedRules,
      })
    }
    if (thresholdChanged) {
      configurationUpdates.push({
        key: 'UserViolationBanThreshold',
        value: values.UserViolationBanThreshold,
      })
    }
    if (windowHoursChanged) {
      configurationUpdates.push({
        key: 'UserViolationBanWindowHours',
        value: values.UserViolationBanWindowHours,
      })
    }

    const enabledUpdate = {
      key: 'UserViolationBanEnabled',
      value: values.UserViolationBanEnabled,
    }

    if (!values.UserViolationBanEnabled && enabledChanged) {
      await updateOption.mutateAsync(enabledUpdate)
    }
    for (const update of configurationUpdates) {
      await updateOption.mutateAsync(update)
    }
    if (values.UserViolationBanEnabled && enabledChanged) {
      await updateOption.mutateAsync(enabledUpdate)
    }
  }

  return (
    <SettingsSection title={t('Violation Ban')}>
      <Form {...form}>
        <SettingsForm onSubmit={form.handleSubmit(onSubmit)}>
          <SettingsPageFormActions
            onSave={form.handleSubmit(onSubmit)}
            isSaving={updateOption.isPending}
            saveLabel='Save violation ban settings'
          />
          <FormField
            control={form.control}
            name='UserViolationBanEnabled'
            render={({ field }) => (
              <SettingsSwitchItem>
                <SettingsSwitchContent>
                  <FormLabel>{t('Enable automatic violation bans')}</FormLabel>
                  <FormDescription>
                    {t(
                      'Automatically disable a user after matching upstream error responses reach the configured threshold.'
                    )}
                  </FormDescription>
                </SettingsSwitchContent>
                <FormControl>
                  <Switch
                    checked={field.value}
                    onCheckedChange={field.onChange}
                  />
                </FormControl>
              </SettingsSwitchItem>
            )}
          />

          <FormField
            control={form.control}
            name='UserViolationBanThreshold'
            render={({ field }) => (
              <FormItem>
                <FormLabel>{t('Ban threshold')}</FormLabel>
                <FormControl>
                  <Input
                    type='number'
                    min={1}
                    max={maxThreshold}
                    step={1}
                    {...field}
                    onChange={(event) =>
                      field.onChange(
                        Number.parseInt(event.target.value, 10) || 0
                      )
                    }
                  />
                </FormControl>
                <FormDescription>
                  {t(
                    'Number of matching responses required to disable a user.'
                  )}
                </FormDescription>
                <FormMessage />
              </FormItem>
            )}
          />

          <FormField
            control={form.control}
            name='UserViolationBanWindowHours'
            render={({ field }) => (
              <FormItem>
                <FormLabel>{t('Sliding window (hours)')}</FormLabel>
                <FormControl>
                  <Input
                    type='number'
                    min={1}
                    max={maxWindowHours}
                    step={1}
                    {...field}
                    onChange={(event) =>
                      field.onChange(
                        Number.parseInt(event.target.value, 10) || 0
                      )
                    }
                  />
                </FormControl>
                <FormDescription>
                  {t(
                    'The counter is cleared after this many hours without another matching response.'
                  )}
                </FormDescription>
                <FormMessage />
              </FormItem>
            )}
          />

          <FormField
            control={form.control}
            name='UserViolationBanRules'
            render={() => (
              <FormItem className='gap-3 lg:col-span-2'>
                <div className='flex flex-wrap items-center justify-between gap-2'>
                  <div className='space-y-0.5'>
                    <FormLabel>{t('Violation rules')}</FormLabel>
                    <FormDescription>
                      {t(
                        'A response counts when its status code and body keyword both match a rule. Any rule can contribute to the same counter.'
                      )}
                    </FormDescription>
                  </div>
                  <Button
                    type='button'
                    variant='outline'
                    size='sm'
                    onClick={() =>
                      rules.append({ statusCode: 502, keyword: '' })
                    }
                    disabled={rules.fields.length >= maxRules}
                  >
                    <Plus data-icon='inline-start' />
                    {t('Add rule')}
                  </Button>
                </div>

                <div className='divide-border border-border divide-y border-y'>
                  {rules.fields.map((rule, index) => (
                    <div
                      key={rule.id}
                      className='grid min-w-0 gap-3 py-3 md:grid-cols-[minmax(8rem,0.35fr)_minmax(0,1fr)_auto] md:items-end'
                    >
                      <FormField
                        control={form.control}
                        name={`UserViolationBanRules.${index}.statusCode`}
                        render={({ field }) => (
                          <FormItem>
                            <FormLabel>{t('Status code')}</FormLabel>
                            <FormControl>
                              <Input
                                type='number'
                                min={100}
                                max={599}
                                step={1}
                                {...field}
                                onChange={(event) =>
                                  field.onChange(
                                    Number.parseInt(event.target.value, 10) || 0
                                  )
                                }
                              />
                            </FormControl>
                            <FormMessage />
                          </FormItem>
                        )}
                      />

                      <FormField
                        control={form.control}
                        name={`UserViolationBanRules.${index}.keyword`}
                        render={({ field }) => (
                          <FormItem>
                            <FormLabel>{t('Response contains')}</FormLabel>
                            <FormControl>
                              <Input
                                maxLength={maxKeywordLength}
                                placeholder={t('Enter text to match')}
                                {...field}
                              />
                            </FormControl>
                            <FormMessage />
                          </FormItem>
                        )}
                      />

                      <Tooltip>
                        <TooltipTrigger
                          render={
                            <Button
                              type='button'
                              variant='ghost'
                              size='icon-sm'
                              onClick={() => rules.remove(index)}
                              aria-label={t('Remove violation rule')}
                            />
                          }
                        >
                          <Trash2 />
                        </TooltipTrigger>
                        <TooltipContent>
                          {t('Remove violation rule')}
                        </TooltipContent>
                      </Tooltip>
                    </div>
                  ))}
                </div>
                <FormMessage />
              </FormItem>
            )}
          />
        </SettingsForm>
      </Form>
    </SettingsSection>
  )
}
