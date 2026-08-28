import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { Pencil, Plus, ReceiptText, Trash2 } from 'lucide-react'
import { useMemo, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { Dialog } from '@/components/dialog'
import { SectionPageLayout } from '@/components/layout'
import { StatusBadge } from '@/components/status-badge'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Checkbox } from '@/components/ui/checkbox'
import { Label } from '@/components/ui/label'
import { RadioGroup, RadioGroupItem } from '@/components/ui/radio-group'
import { Tabs, TabsList, TabsTrigger } from '@/components/ui/tabs'
import { formatBillingCurrencyFromUSD } from '@/lib/currency'
import { formatTimestampToDate } from '@/lib/format'

import {
  createInvoiceApplication,
  deleteInvoiceTitle,
  getInvoiceApplications,
  getInvoiceOrders,
  getInvoiceTitles,
} from './api'
import { InvoiceTitleDialog } from './invoice-title-dialog'
import type { InvoiceStatus, InvoiceTitle } from './types'

const statusVariant = {
  unissued: 'neutral',
  issuing: 'warning',
  issued: 'success',
} as const

export function UserInvoicesPage() {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const [status, setStatus] = useState<InvoiceStatus>('unissued')
  const [selected, setSelected] = useState<Set<number>>(new Set())
  const [titleID, setTitleID] = useState<number | null>(null)
  const [titleManagementOpen, setTitleManagementOpen] = useState(false)
  const [titleDialogOpen, setTitleDialogOpen] = useState(false)
  const [editingTitle, setEditingTitle] = useState<InvoiceTitle | null>(null)
  const [confirmOpen, setConfirmOpen] = useState(false)
  const ordersQuery = useQuery({
    queryKey: ['invoice-orders', status],
    queryFn: () => getInvoiceOrders(status),
  })
  const applicationsQuery = useQuery({
    queryKey: ['invoice-applications', status],
    queryFn: () => getInvoiceApplications(status),
  })
  const titlesQuery = useQuery({
    queryKey: ['invoice-titles'],
    queryFn: getInvoiceTitles,
  })
  const orders = useMemo(
    () => ordersQuery.data?.data?.items ?? [],
    [ordersQuery.data?.data?.items]
  )
  const applications = useMemo(
    () => applicationsQuery.data?.data?.items ?? [],
    [applicationsQuery.data?.data?.items]
  )
  const titles = useMemo(
    () => titlesQuery.data?.data?.items ?? [],
    [titlesQuery.data?.data?.items]
  )
  const eligibleOrders = useMemo(
    () => (status === 'unissued' ? orders : []),
    [orders, status]
  )
  const selectedOrders = useMemo(
    () => eligibleOrders.filter((order) => selected.has(order.id)),
    [eligibleOrders, selected]
  )
  const total = selectedOrders.reduce((sum, order) => sum + order.money, 0)
  const deleteMutation = useMutation({
    mutationFn: (id: number) => deleteInvoiceTitle(id),
    onSuccess: async (response) => {
      if (!response.success) {
        toast.error(response.message || t('Failed to delete invoice title'))
        return
      }
      await queryClient.invalidateQueries({ queryKey: ['invoice-titles'] })
      toast.success(t('Invoice title deleted'))
    },
  })
  const createMutation = useMutation({
    mutationFn: () =>
      createInvoiceApplication(
        titleID || 0,
        selectedOrders.map((order) => order.id)
      ),
    onSuccess: async (response) => {
      if (!response.success) {
        toast.error(response.message || t('Failed to submit invoice request'))
        return
      }
      setSelected(new Set())
      setConfirmOpen(false)
      await Promise.all([
        queryClient.invalidateQueries({ queryKey: ['invoice-orders'] }),
        queryClient.invalidateQueries({ queryKey: ['invoice-applications'] }),
      ])
      toast.success(t('Invoice request submitted'))
    },
  })

  const openConfirmation = () => {
    if (titles.length === 0) {
      setTitleDialogOpen(true)
      return
    }
    setTitleID(titles[0].id)
    setConfirmOpen(true)
  }

  return (
    <>
      <SectionPageLayout>
        <SectionPageLayout.Title>{t('Invoices')}</SectionPageLayout.Title>
        <SectionPageLayout.Actions>
          <Button
            variant='outline'
            onClick={() => setTitleManagementOpen(true)}
          >
            <ReceiptText data-icon='inline-start' />
            {t('Invoice title management')}
          </Button>
        </SectionPageLayout.Actions>
        <SectionPageLayout.Content>
          <div className='mx-auto flex max-w-6xl flex-col gap-4'>
            <Tabs
              value={status}
              onValueChange={(value) => {
                if (value) {
                  setStatus(value as InvoiceStatus)
                  setSelected(new Set())
                }
              }}
            >
              <TabsList>
                <TabsTrigger value='unissued'>{t('Not invoiced')}</TabsTrigger>
                <TabsTrigger value='issuing'>
                  {t('Pending invoice')}
                </TabsTrigger>
                <TabsTrigger value='issued'>{t('Invoiced')}</TabsTrigger>
              </TabsList>
            </Tabs>

            {status === 'unissued' && eligibleOrders.length > 0 && (
              <div className='flex flex-wrap items-center justify-between gap-3 border-y py-3'>
                <Label className='flex items-center gap-2'>
                  <Checkbox
                    checked={selected.size === eligibleOrders.length}
                    onCheckedChange={(checked) =>
                      setSelected(
                        checked
                          ? new Set(eligibleOrders.map((order) => order.id))
                          : new Set()
                      )
                    }
                  />
                  {t('Select all available orders')}
                </Label>
                <div className='flex items-center gap-3'>
                  <span className='text-muted-foreground text-sm'>
                    {t('{{count}} orders selected', { count: selected.size })} ·{' '}
                    <strong className='text-foreground'>
                      {formatBillingCurrencyFromUSD(total, {
                        digitsLarge: 2,
                        digitsSmall: 2,
                        abbreviate: false,
                      })}
                    </strong>
                  </span>
                  <Button
                    disabled={selected.size === 0}
                    onClick={openConfirmation}
                  >
                    <ReceiptText data-icon='inline-start' />
                    {t('Request invoice')}
                  </Button>
                </div>
              </div>
            )}

            {status === 'unissued' ? (
              <div className='divide-y rounded-lg border'>
                {eligibleOrders.map((order) => (
                  <label
                    key={order.id}
                    className='hover:bg-muted/40 flex cursor-pointer items-center gap-3 p-4'
                  >
                    <Checkbox
                      checked={selected.has(order.id)}
                      onCheckedChange={(checked) =>
                        setSelected((current) => {
                          const next = new Set(current)
                          if (checked) next.add(order.id)
                          else next.delete(order.id)
                          return next
                        })
                      }
                    />
                    <div className='min-w-0 flex-1'>
                      <div className='truncate font-mono text-sm'>
                        {order.trade_no}
                      </div>
                      <div className='text-muted-foreground text-xs'>
                        {formatTimestampToDate(order.complete_time)}
                      </div>
                    </div>
                    <strong className='tabular-nums'>
                      {formatBillingCurrencyFromUSD(order.money, {
                        digitsLarge: 2,
                        digitsSmall: 2,
                        abbreviate: false,
                      })}
                    </strong>
                  </label>
                ))}
                {!ordersQuery.isLoading && eligibleOrders.length === 0 && (
                  <div className='text-muted-foreground py-16 text-center'>
                    {t('No invoiceable orders')}
                  </div>
                )}
              </div>
            ) : (
              <div className='grid gap-3 md:grid-cols-2'>
                {applications.map((application) => (
                  <Card key={application.id} className='rounded-lg'>
                    <CardHeader>
                      <CardTitle className='flex items-center justify-between gap-2'>
                        <span>
                          {t('Invoice request')} #{application.id}
                        </span>
                        <StatusBadge
                          label={t(
                            application.status === 'issued'
                              ? 'Invoiced'
                              : 'Pending invoice'
                          )}
                          variant={statusVariant[application.status]}
                          copyable={false}
                        />
                      </CardTitle>
                    </CardHeader>
                    <CardContent className='grid gap-2 text-sm'>
                      <div className='flex justify-between'>
                        <span className='text-muted-foreground'>
                          {t('Invoice title')}
                        </span>
                        <span>{application.title_name}</span>
                      </div>
                      <div className='flex justify-between'>
                        <span className='text-muted-foreground'>
                          {t('Invoice email')}
                        </span>
                        <span>{application.recipient_email}</span>
                      </div>
                      <div className='flex justify-between'>
                        <span className='text-muted-foreground'>
                          {t('Total amount')}
                        </span>
                        <strong>
                          {formatBillingCurrencyFromUSD(
                            application.total_money,
                            {
                              digitsLarge: 2,
                              digitsSmall: 2,
                              abbreviate: false,
                            }
                          )}
                        </strong>
                      </div>
                    </CardContent>
                  </Card>
                ))}
                {!applicationsQuery.isLoading && applications.length === 0 && (
                  <div className='text-muted-foreground col-span-full py-16 text-center'>
                    {t('No invoice requests')}
                  </div>
                )}
              </div>
            )}
          </div>
        </SectionPageLayout.Content>
      </SectionPageLayout>

      <Dialog
        open={titleManagementOpen}
        onOpenChange={setTitleManagementOpen}
        title={t('Invoice title management')}
        description={t(
          'Manage your saved invoice titles, including adding, editing, and deleting them.'
        )}
        footer={
          <Button
            variant='outline'
            onClick={() => setTitleManagementOpen(false)}
          >
            {t('Close')}
          </Button>
        }
      >
        <div className='space-y-3'>
          <Button
            onClick={() => {
              setEditingTitle(null)
              setTitleManagementOpen(false)
              setTitleDialogOpen(true)
            }}
          >
            <Plus data-icon='inline-start' />
            {t('Add invoice title')}
          </Button>
          {titles.length === 0 ? (
            <div className='text-muted-foreground rounded-lg border py-10 text-center text-sm'>
              {t('No saved invoice titles')}
            </div>
          ) : (
            <div className='divide-y rounded-lg border'>
              {titles.map((title) => (
                <div
                  key={title.id}
                  className='flex items-start justify-between gap-3 p-4 text-sm'
                >
                  <div className='min-w-0 flex-1 space-y-1'>
                    <div className='font-medium'>{title.name}</div>
                    <dl className='text-muted-foreground grid gap-1 text-xs sm:grid-cols-2'>
                      <div className='min-w-0'>
                        <dt className='inline'>{t('Tax number')}: </dt>
                        <dd className='inline break-all'>{title.tax_number}</dd>
                      </div>
                      <div className='min-w-0'>
                        <dt className='inline'>{t('Invoice email')}: </dt>
                        <dd className='inline break-all'>{title.email}</dd>
                      </div>
                      <div className='min-w-0 sm:col-span-2'>
                        <dt className='inline'>{t('Registered address')}: </dt>
                        <dd className='inline break-words'>{title.address}</dd>
                      </div>
                    </dl>
                  </div>
                  <div className='flex shrink-0 gap-1'>
                    <Button
                      aria-label={t('Edit invoice title')}
                      variant='ghost'
                      size='icon'
                      onClick={() => {
                        setEditingTitle(title)
                        setTitleManagementOpen(false)
                        setTitleDialogOpen(true)
                      }}
                    >
                      <Pencil />
                    </Button>
                    <Button
                      aria-label={t('Delete invoice title')}
                      variant='ghost'
                      size='icon'
                      onClick={() => deleteMutation.mutate(title.id)}
                      disabled={deleteMutation.isPending}
                    >
                      <Trash2 />
                    </Button>
                  </div>
                </div>
              ))}
            </div>
          )}
        </div>
      </Dialog>

      <InvoiceTitleDialog
        open={titleDialogOpen}
        onOpenChange={setTitleDialogOpen}
        title={editingTitle}
      />
      <Dialog
        open={confirmOpen}
        onOpenChange={setConfirmOpen}
        title={t('Confirm invoice request')}
        description={t('Choose the invoice title for these orders.')}
        footer={
          <>
            <Button variant='outline' onClick={() => setConfirmOpen(false)}>
              {t('Cancel')}
            </Button>
            <Button
              disabled={!titleID || createMutation.isPending}
              onClick={() => createMutation.mutate()}
            >
              {createMutation.isPending ? t('Submitting...') : t('Confirm')}
            </Button>
          </>
        }
      >
        <div className='space-y-4'>
          <div className='rounded-lg border p-4 text-center'>
            <div className='text-muted-foreground text-sm'>
              {t('Total amount')}
            </div>
            <div className='mt-1 text-2xl font-semibold tabular-nums'>
              {formatBillingCurrencyFromUSD(total, {
                digitsLarge: 2,
                digitsSmall: 2,
                abbreviate: false,
              })}
            </div>
          </div>
          <div className='space-y-2'>
            <Label>{t('Invoice title')}</Label>
            <RadioGroup
              aria-label={t('Invoice title')}
              value={titleID ? String(titleID) : ''}
              onValueChange={(value) =>
                setTitleID(value ? Number(value) : null)
              }
              className='max-h-72 overflow-y-auto rounded-lg border p-2'
            >
              {titles.map((title) => (
                <label
                  key={title.id}
                  className='hover:bg-muted/50 flex cursor-pointer items-start gap-3 rounded-md p-3 transition-colors'
                >
                  <RadioGroupItem
                    value={String(title.id)}
                    aria-label={title.name}
                    className='mt-1'
                  />
                  <span className='min-w-0 flex-1 space-y-1 text-sm'>
                    <span className='block font-medium break-words'>
                      {title.name}
                    </span>
                    <span className='text-muted-foreground grid gap-1 text-xs sm:grid-cols-2'>
                      <span className='min-w-0 break-all'>
                        {t('Tax number')}: {title.tax_number}
                      </span>
                      <span className='min-w-0 break-all'>
                        {t('Invoice email')}: {title.email}
                      </span>
                      <span className='min-w-0 break-words sm:col-span-2'>
                        {t('Registered address')}: {title.address}
                      </span>
                    </span>
                  </span>
                </label>
              ))}
            </RadioGroup>
          </div>
        </div>
      </Dialog>
    </>
  )
}
