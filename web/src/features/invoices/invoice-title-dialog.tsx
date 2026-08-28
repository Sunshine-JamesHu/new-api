import { useMutation, useQueryClient } from '@tanstack/react-query'
import { useEffect, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { Dialog } from '@/components/dialog'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'

import { createInvoiceTitle, updateInvoiceTitle } from './api'
import type { InvoiceTitle } from './types'

interface InvoiceTitleDialogProps {
  open: boolean
  onOpenChange: (open: boolean) => void
  title?: InvoiceTitle | null
}

export function InvoiceTitleDialog(props: InvoiceTitleDialogProps) {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const [name, setName] = useState('')
  const [taxNumber, setTaxNumber] = useState('')
  const [address, setAddress] = useState('')
  const [email, setEmail] = useState('')
  useEffect(() => {
    setName(props.title?.name || '')
    setTaxNumber(props.title?.tax_number || '')
    setAddress(props.title?.address || '')
    setEmail(props.title?.email || '')
  }, [props.title, props.open])
  const mutation = useMutation({
    mutationFn: () =>
      props.title
        ? updateInvoiceTitle(props.title.id, {
            name,
            tax_number: taxNumber,
            address,
            email,
          })
        : createInvoiceTitle({ name, tax_number: taxNumber, address, email }),
    onSuccess: async (response) => {
      if (!response.success) {
        toast.error(response.message || t('Failed to save invoice title'))
        return
      }
      await queryClient.invalidateQueries({ queryKey: ['invoice-titles'] })
      setName('')
      setTaxNumber('')
      setAddress('')
      setEmail('')
      props.onOpenChange(false)
      toast.success(t('Invoice title saved'))
    },
  })
  const valid =
    name.trim() &&
    taxNumber.trim() &&
    address.trim() &&
    /^\S+@\S+\.\S+$/.test(email)

  return (
    <Dialog
      open={props.open}
      onOpenChange={props.onOpenChange}
      title={props.title ? t('Edit invoice title') : t('Add invoice title')}
      description={t(
        'This information will be used on your invoice and delivery email.'
      )}
      footer={
        <>
          <Button variant='outline' onClick={() => props.onOpenChange(false)}>
            {t('Cancel')}
          </Button>
          <Button
            disabled={!valid || mutation.isPending}
            onClick={() => mutation.mutate()}
          >
            {mutation.isPending ? t('Saving...') : t('Save')}
          </Button>
        </>
      }
    >
      <div className='space-y-4'>
        <div className='space-y-1.5'>
          <Label htmlFor='invoice-name'>{t('Company name')}</Label>
          <Input
            id='invoice-name'
            value={name}
            onChange={(event) => setName(event.target.value)}
            placeholder={t('Enter the legal company name')}
          />
        </div>
        <div className='space-y-1.5'>
          <Label htmlFor='invoice-tax'>{t('Tax number')}</Label>
          <Input
            id='invoice-tax'
            value={taxNumber}
            onChange={(event) => setTaxNumber(event.target.value)}
            placeholder={t('Enter the company tax identification number')}
          />
        </div>
        <div className='space-y-1.5'>
          <Label htmlFor='invoice-address'>{t('Registered address')}</Label>
          <Input
            id='invoice-address'
            value={address}
            onChange={(event) => setAddress(event.target.value)}
            placeholder={t('Enter the registered company address')}
          />
        </div>
        <div className='space-y-1.5'>
          <Label htmlFor='invoice-email'>{t('Invoice email')}</Label>
          <Input
            id='invoice-email'
            type='email'
            value={email}
            onChange={(event) => setEmail(event.target.value)}
            placeholder={t('Enter the email address for invoice delivery')}
          />
        </div>
      </div>
    </Dialog>
  )
}
