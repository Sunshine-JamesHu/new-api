import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { FileText, Mail, Upload } from 'lucide-react'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { Dialog } from '@/components/dialog'
import { SectionPageLayout } from '@/components/layout'
import { StatusBadge } from '@/components/status-badge'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Tabs, TabsList, TabsTrigger } from '@/components/ui/tabs'
import { Textarea } from '@/components/ui/textarea'
import { formatBillingCurrencyFromUSD } from '@/lib/currency'

import {
  getInvoiceApplicationDetail,
  getInvoiceApplications,
  getInvoiceEmailTemplate,
  issueInvoice,
  updateInvoiceEmailTemplate,
} from './api'
import type { InvoiceApplication, InvoiceStatus } from './types'

export function AdminInvoicesPage() {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const [status, setStatus] = useState<InvoiceStatus | 'all'>('issuing')
  const [keyword, setKeyword] = useState('')
  const [detailID, setDetailID] = useState<number | null>(null)
  const [issueTarget, setIssueTarget] = useState<InvoiceApplication | null>(
    null
  )
  const [file, setFile] = useState<File | null>(null)
  const [templateOpen, setTemplateOpen] = useState(false)
  const applicationsQuery = useQuery({
    queryKey: ['admin-invoices', status, keyword],
    queryFn: () => getInvoiceApplications(status, true, 1, 100, keyword),
  })
  const detailQuery = useQuery({
    queryKey: ['admin-invoice-detail', detailID],
    queryFn: () => getInvoiceApplicationDetail(detailID || 0, true),
    enabled: detailID !== null,
  })
  const templateQuery = useQuery({
    queryKey: ['invoice-template'],
    queryFn: getInvoiceEmailTemplate,
  })
  const [subject, setSubject] = useState('')
  const [body, setBody] = useState('')
  const applications = applicationsQuery.data?.data?.items || []
  const detail = detailQuery.data?.data
  const issueMutation = useMutation({
    mutationFn: () => issueInvoice(issueTarget?.id || 0, file as File),
    onSuccess: async (response) => {
      if (!response.success) {
        toast.error(response.message || t('Failed to issue invoice'))
        return
      }
      setIssueTarget(null)
      setFile(null)
      await queryClient.invalidateQueries({ queryKey: ['admin-invoices'] })
      toast.success(t('Invoice sent successfully'))
    },
  })
  const templateMutation = useMutation({
    mutationFn: () => updateInvoiceEmailTemplate({ subject, body }),
    onSuccess: async (response) => {
      if (!response.success) {
        toast.error(response.message || t('Failed to save email template'))
        return
      }
      await queryClient.invalidateQueries({ queryKey: ['invoice-template'] })
      setTemplateOpen(false)
      toast.success(t('Email template saved'))
    },
  })
  const openTemplate = () => {
    setSubject(templateQuery.data?.data?.subject || '')
    setBody(templateQuery.data?.data?.body || '')
    setTemplateOpen(true)
  }
  const acceptFile = (candidate?: File) => {
    if (
      !candidate ||
      (candidate.type && candidate.type !== 'application/pdf') ||
      !candidate.name.toLowerCase().endsWith('.pdf') ||
      candidate.size <= 0 ||
      candidate.size > 10 * 1024 * 1024
    ) {
      toast.error(t('Choose a PDF file no larger than 10 MiB'))
      return
    }
    setFile(candidate)
  }

  return (
    <>
      <SectionPageLayout>
        <SectionPageLayout.Title>
          {t('Invoice management')}
        </SectionPageLayout.Title>
        <SectionPageLayout.Actions>
          <Button variant='outline' onClick={openTemplate}>
            <Mail data-icon='inline-start' />
            {t('Email template')}
          </Button>
        </SectionPageLayout.Actions>
        <SectionPageLayout.Content>
          <div className='mx-auto max-w-7xl space-y-4'>
            <Tabs
              value={status}
              onValueChange={(value) =>
                value && setStatus(value as InvoiceStatus | 'all')
              }
            >
              <TabsList>
                <TabsTrigger value='issuing'>
                  {t('Pending invoice')}
                </TabsTrigger>
                <TabsTrigger value='issued'>{t('Invoiced')}</TabsTrigger>
                <TabsTrigger value='all'>{t('All')}</TabsTrigger>
              </TabsList>
            </Tabs>
            <div className='flex gap-2'>
              <Input
                aria-label={t('Search')}
                value={keyword}
                onChange={(event) => setKeyword(event.target.value)}
                placeholder={t('Search')}
              />
              <Button
                variant='outline'
                onClick={() => setKeyword('')}
                disabled={!keyword}
              >
                {t('Clear')}
              </Button>
            </div>
            <div className='divide-y rounded-lg border'>
              {applications.map((application) => (
                <div
                  key={application.id}
                  className='grid gap-3 p-4 md:grid-cols-[minmax(0,1fr)_minmax(12rem,auto)_auto] md:items-center'
                >
                  <button
                    type='button'
                    className='min-w-0 text-left'
                    onClick={() => setDetailID(application.id)}
                  >
                    <div className='font-medium'>{application.title_name}</div>
                    <div className='text-muted-foreground truncate text-sm'>
                      {application.recipient_email}
                    </div>
                  </button>
                  <div>
                    <div className='font-semibold tabular-nums'>
                      {formatBillingCurrencyFromUSD(application.total_money, {
                        digitsLarge: 2,
                        digitsSmall: 2,
                        abbreviate: false,
                      })}
                    </div>
                    <StatusBadge
                      label={t(
                        application.status === 'issued'
                          ? 'Invoiced'
                          : 'Pending invoice'
                      )}
                      variant={
                        application.status === 'issued' ? 'success' : 'warning'
                      }
                      copyable={false}
                    />
                  </div>
                  <div className='flex gap-2'>
                    <Button
                      variant='outline'
                      size='sm'
                      onClick={() => setDetailID(application.id)}
                    >
                      <FileText data-icon='inline-start' />
                      {t('Details')}
                    </Button>
                    {application.status !== 'issued' && (
                      <Button
                        size='sm'
                        onClick={() => setIssueTarget(application)}
                      >
                        <Upload data-icon='inline-start' />
                        {t('Issue invoice')}
                      </Button>
                    )}
                  </div>
                </div>
              ))}
              {!applicationsQuery.isLoading && applications.length === 0 && (
                <div className='text-muted-foreground py-16 text-center'>
                  {t('No invoice requests')}
                </div>
              )}
            </div>
          </div>
        </SectionPageLayout.Content>
      </SectionPageLayout>

      <Dialog
        open={detailID !== null}
        onOpenChange={(open) => !open && setDetailID(null)}
        title={t('Invoice details')}
      >
        {detail && (
          <div className='space-y-4 text-sm'>
            <div className='grid gap-2 rounded-lg border p-4'>
              <div>
                <span className='text-muted-foreground'>
                  {t('Invoice title')}:{' '}
                </span>
                {detail.application.title_name}
              </div>
              <div>
                <span className='text-muted-foreground'>
                  {t('Tax number')}:{' '}
                </span>
                {detail.application.title_tax_number}
              </div>
              <div>
                <span className='text-muted-foreground'>
                  {t('Registered address')}:{' '}
                </span>
                {detail.application.title_address}
              </div>
              <div>
                <span className='text-muted-foreground'>
                  {t('Invoice email')}:{' '}
                </span>
                {detail.application.recipient_email}
              </div>
              <div>
                <span className='text-muted-foreground'>
                  {t('Total amount')}:{' '}
                </span>
                <strong>${detail.application.total_money.toFixed(2)}</strong>
              </div>
              <div>
                <span className='text-muted-foreground'>{t('Status')}: </span>
                {t(
                  detail.application.status === 'issued'
                    ? 'Invoiced'
                    : 'Pending invoice'
                )}
              </div>
              {detail.application.pdf_file_name && (
                <div>
                  <span className='text-muted-foreground'>
                    {t('PDF file')}:{' '}
                  </span>
                  {detail.application.pdf_file_name} (
                  {detail.application.pdf_size} bytes)
                </div>
              )}
              {detail.application.last_error && (
                <div className='text-destructive'>
                  {detail.application.last_error}
                </div>
              )}
            </div>
            <div className='divide-y rounded-lg border'>
              {detail.orders.map((order) => (
                <div key={order.id} className='flex justify-between gap-3 p-3'>
                  <code>{order.trade_no}</code>
                  <strong>${order.money.toFixed(2)}</strong>
                </div>
              ))}
            </div>
          </div>
        )}
      </Dialog>

      <Dialog
        open={issueTarget !== null}
        onOpenChange={(open) => {
          if (!open) {
            setIssueTarget(null)
            setFile(null)
          }
        }}
        title={t('Issue invoice')}
        description={t(
          'The linked orders will be marked as invoiced only after SMTP confirms delivery.'
        )}
        footer={
          <>
            <Button variant='outline' onClick={() => setIssueTarget(null)}>
              {t('Cancel')}
            </Button>
            <Button
              disabled={!file || issueMutation.isPending}
              onClick={() => issueMutation.mutate()}
            >
              {issueMutation.isPending ? t('Sending...') : t('Send invoice')}
            </Button>
          </>
        }
      >
        <label
          className='border-muted-foreground/40 hover:bg-muted/40 flex min-h-40 cursor-pointer flex-col items-center justify-center gap-2 rounded-lg border-2 border-dashed p-6 text-center'
          onDragOver={(event) => event.preventDefault()}
          onDrop={(event) => {
            event.preventDefault()
            acceptFile(event.dataTransfer.files[0])
          }}
        >
          <Upload className='text-muted-foreground size-8' />
          <span className='font-medium'>
            {file?.name || t('Drop a PDF here or choose a file')}
          </span>
          <span className='text-muted-foreground text-xs'>
            {t('PDF only, up to 10 MiB')}
          </span>
          <Input
            className='sr-only'
            type='file'
            accept='application/pdf,.pdf'
            onChange={(event) => acceptFile(event.target.files?.[0])}
          />
        </label>
      </Dialog>

      <Dialog
        open={templateOpen}
        onOpenChange={setTemplateOpen}
        title={t('Invoice email template')}
        description={t(
          'Available placeholders: {{system_name}}, {{invoice_id}}, {{recipient_email}}, {{title_name}}, {{total_money}}, {{orders}}'
        )}
        footer={
          <>
            <Button variant='outline' onClick={() => setTemplateOpen(false)}>
              {t('Cancel')}
            </Button>
            <Button
              disabled={
                !subject.trim() || !body.trim() || templateMutation.isPending
              }
              onClick={() => templateMutation.mutate()}
            >
              {templateMutation.isPending ? t('Saving...') : t('Save')}
            </Button>
          </>
        }
      >
        <div className='space-y-4'>
          <div className='space-y-1.5'>
            <Label htmlFor='invoice-subject'>{t('Email subject')}</Label>
            <Input
              id='invoice-subject'
              value={subject}
              onChange={(event) => setSubject(event.target.value)}
            />
          </div>
          <div className='space-y-1.5'>
            <Label htmlFor='invoice-body'>{t('Email body')}</Label>
            <Textarea
              id='invoice-body'
              rows={10}
              value={body}
              onChange={(event) => setBody(event.target.value)}
            />
          </div>
        </div>
      </Dialog>
    </>
  )
}
