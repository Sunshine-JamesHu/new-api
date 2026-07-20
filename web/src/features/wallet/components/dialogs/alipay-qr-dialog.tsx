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
import { QRCodeSVG } from 'qrcode.react'
import { useTranslation } from 'react-i18next'

import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'

type AlipayQrDialogProps = {
  qrCode: string
  onOpenChange: (open: boolean) => void
}

export function AlipayQrDialog({ qrCode, onOpenChange }: AlipayQrDialogProps) {
  const { t } = useTranslation()
  const open = qrCode.length > 0

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className='sm:max-w-sm'>
        <DialogHeader>
          <DialogTitle>{t('Alipay QR payment')}</DialogTitle>
          <DialogDescription>
            {t('Use Alipay to scan the QR code and complete payment.')}
          </DialogDescription>
        </DialogHeader>
        <div className='flex justify-center py-4'>
          <div className='rounded-lg border bg-white p-4'>
            <QRCodeSVG value={qrCode} size={220} />
          </div>
        </div>
      </DialogContent>
    </Dialog>
  )
}
