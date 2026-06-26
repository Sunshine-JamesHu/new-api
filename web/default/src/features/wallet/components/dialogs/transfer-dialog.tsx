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
import { useState, useEffect } from 'react'
import { Loader2 } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import {
  formatNumber,
  formatQuota,
  parseQuotaFromDollars,
  quotaUnitsToDollars,
} from '@/lib/format'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Tabs, TabsList, TabsTrigger } from '@/components/ui/tabs'
import { Dialog } from '@/components/dialog'
import { QUOTA_PER_DOLLAR } from '../../constants'

interface TransferDialogProps {
  open: boolean
  onOpenChange: (open: boolean) => void
  onConfirm: (amount: number) => Promise<boolean>
  availableQuota: number
  transferring: boolean
}

export function TransferDialog({
  open,
  onOpenChange,
  onConfirm,
  availableQuota,
  transferring,
}: TransferDialogProps) {
  const { t } = useTranslation()
  const [transferMode, setTransferMode] = useState<'amount' | 'tokens'>(
    'amount'
  )
  const [amount, setAmount] = useState(quotaUnitsToDollars(QUOTA_PER_DOLLAR))
  const [quota, setQuota] = useState(QUOTA_PER_DOLLAR)
  const quotaToTransfer =
    transferMode === 'amount' ? parseQuotaFromDollars(amount) : quota
  const maxAmount = quotaUnitsToDollars(availableQuota)

  useEffect(() => {
    if (!open) {
      return
    }

    // eslint-disable-next-line react-hooks/set-state-in-effect
    setTransferMode('amount')
    // eslint-disable-next-line react-hooks/set-state-in-effect
    setAmount(quotaUnitsToDollars(QUOTA_PER_DOLLAR))
    // eslint-disable-next-line react-hooks/set-state-in-effect
    setQuota(QUOTA_PER_DOLLAR)
  }, [open])

  const handleConfirm = async () => {
    const success = await onConfirm(quotaToTransfer)
    if (success) {
      onOpenChange(false)
    }
  }

  return (
    <Dialog
      open={open}
      onOpenChange={onOpenChange}
      title={t('Transfer Rewards')}
      description={t('Move affiliate rewards to your main balance')}
      contentClassName='max-sm:w-[calc(100vw-1.5rem)] sm:max-w-md'
      titleClassName='text-xl font-semibold'
      footerClassName='grid grid-cols-2 gap-2 sm:flex'
      contentHeight='auto'
      bodyClassName='space-y-4'
      footer={
        <>
          <Button
            variant='outline'
            onClick={() => onOpenChange(false)}
            disabled={transferring}
          >
            {t('Cancel')}
          </Button>
          <Button
            onClick={handleConfirm}
            disabled={
              transferring ||
              quotaToTransfer < QUOTA_PER_DOLLAR ||
              quotaToTransfer > availableQuota
            }
          >
            {transferring && <Loader2 className='mr-2 h-4 w-4 animate-spin' />}
            {t('Transfer')}
          </Button>
        </>
      }
    >
      <div className='space-y-4 py-3 sm:space-y-6 sm:py-4'>
        <div className='space-y-2'>
          <Label className='text-muted-foreground text-xs font-medium tracking-wider uppercase'>
            {t('Available Rewards')}
          </Label>
          <div className='text-2xl font-semibold'>
            {formatQuota(availableQuota)}
          </div>
        </div>

        <Tabs
          value={transferMode}
          onValueChange={(value) =>
            setTransferMode(value === 'tokens' ? 'tokens' : 'amount')
          }
        >
          <TabsList className='grid w-full grid-cols-2'>
            <TabsTrigger value='amount'>{t('By amount')}</TabsTrigger>
            <TabsTrigger value='tokens'>{t('By tokens')}</TabsTrigger>
          </TabsList>
        </Tabs>

        <div className='space-y-3'>
          <Label
            htmlFor='transfer-amount'
            className='text-muted-foreground text-xs font-medium tracking-wider uppercase'
          >
            {transferMode === 'amount'
              ? t('Transfer Amount')
              : t('Transfer Tokens')}
          </Label>
          <Input
            id='transfer-amount'
            type='number'
            value={transferMode === 'amount' ? amount : quota}
            onChange={(e) => {
              const value = Number(e.target.value)
              if (transferMode === 'amount') {
                setAmount(value)
                return
              }
              setQuota(value)
            }}
            min={
              transferMode === 'amount'
                ? quotaUnitsToDollars(QUOTA_PER_DOLLAR)
                : QUOTA_PER_DOLLAR
            }
            max={transferMode === 'amount' ? maxAmount : availableQuota}
            step={
              transferMode === 'amount'
                ? quotaUnitsToDollars(QUOTA_PER_DOLLAR)
                : QUOTA_PER_DOLLAR
            }
            className='font-mono text-lg'
          />
          <p className='text-muted-foreground text-xs'>
            {t('Minimum:')} {formatQuota(QUOTA_PER_DOLLAR)}
          </p>
          <p className='text-muted-foreground text-xs'>
            {t('Will transfer')}: {formatQuota(quotaToTransfer)} (
            {formatNumber(quotaToTransfer)} {t('tokens')})
          </p>
        </div>
      </div>
    </Dialog>
  )
}
