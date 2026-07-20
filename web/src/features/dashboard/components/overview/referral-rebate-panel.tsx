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
import { AffiliateIcon, ArrowRight01Icon } from '@hugeicons/core-free-icons'
import { HugeiconsIcon } from '@hugeicons/react'
import { Link } from '@tanstack/react-router'
import { useTranslation } from 'react-i18next'

import { CopyButton } from '@/components/copy-button'
import {
  CardStaggerContainer,
  CardStaggerItem,
} from '@/components/page-transition'
import { Button } from '@/components/ui/button'
import {
  Card,
  CardAction,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from '@/components/ui/card'
import { IconBadge } from '@/components/ui/icon-badge'
import { Input } from '@/components/ui/input'
import { generateAffiliateLink } from '@/features/wallet/lib/affiliate'
import { useStatus } from '@/hooks/use-status'
import { useAuthStore } from '@/stores/auth-store'

export function ReferralRebatePanel() {
  const { t } = useTranslation()
  const affCode = useAuthStore((state) => state.auth.user?.aff_code)
  const { status, loading } = useStatus()
  const rebateRate = Number(status?.affiliate_rebate_rate ?? 0)
  const rebateEnabled = status?.affiliate_rebate_enabled === true

  if (loading || !rebateEnabled || rebateRate <= 0 || !affCode) {
    return null
  }

  const affiliateLink = generateAffiliateLink(affCode)
  const formattedRate = rebateRate.toFixed(2).replace(/\.?0+$/, '')

  return (
    <CardStaggerContainer>
      <CardStaggerItem>
        <Card
          size='sm'
          className='border-chart-3/40 bg-chart-3/[0.04] ring-chart-3/20 dark:bg-chart-3/[0.08] border shadow-sm'
        >
          <CardHeader className='border-chart-3/20 border-b has-data-[slot=card-action]:grid-cols-1 sm:has-data-[slot=card-action]:grid-cols-[1fr_auto]'>
            <div className='flex min-w-0 items-start gap-3'>
              <IconBadge tone='chart-3' size='title'>
                <HugeiconsIcon icon={AffiliateIcon} strokeWidth={2} />
              </IconBadge>
              <div className='min-w-0'>
                <CardTitle>{t('Referral Program')}</CardTitle>
                <CardDescription>
                  {t('Invite friends and earn {{rate}}% back', {
                    rate: formattedRate,
                  })}
                </CardDescription>
              </div>
            </div>
            <CardAction className='col-start-1 row-span-1 row-start-2 mt-1 justify-self-end sm:col-start-2 sm:row-span-2 sm:row-start-1 sm:mt-0'>
              <Button
                variant='outline'
                size='sm'
                className='bg-background/80'
                render={<Link to='/wallet' />}
                nativeButton={false}
              >
                {t('View reward details')}
                <HugeiconsIcon
                  icon={ArrowRight01Icon}
                  data-icon='inline-end'
                  strokeWidth={2}
                />
              </Button>
            </CardAction>
          </CardHeader>

          <CardContent className='grid gap-4 lg:grid-cols-[minmax(22rem,0.8fr)_minmax(0,1.2fr)] lg:items-center'>
            <div className='flex min-w-0 flex-col gap-1 lg:border-r lg:pr-5'>
              <span className='text-muted-foreground text-xs font-medium'>
                {t('Rebate percentage')}
              </span>
              <div className='flex flex-wrap items-end gap-x-6 gap-y-1'>
                <span className='font-mono text-3xl font-semibold tabular-nums'>
                  {formattedRate}%
                </span>
                <p className='text-foreground/80 min-w-0 pb-1 text-sm leading-relaxed font-medium'>
                  {t(
                    'Rewards are confirmed as your referrals use their topped-up balance.'
                  )}
                </p>
              </div>
            </div>

            <div className='grid min-w-0 gap-2.5'>
              <div className='flex min-w-0 flex-col gap-1.5'>
                <label
                  htmlFor='dashboard-invite-code'
                  className='text-xs font-medium'
                >
                  {t('Your invite code')}
                </label>
                <div className='flex min-w-0 items-center gap-2'>
                  <Input
                    id='dashboard-invite-code'
                    value={affCode}
                    readOnly
                    className='min-w-0 font-mono'
                  />
                  <CopyButton
                    value={affCode}
                    variant='outline'
                    tooltip={t('Copy invite code')}
                    aria-label={t('Copy invite code')}
                  />
                </div>
              </div>

              <div className='flex min-w-0 flex-col gap-1.5'>
                <label
                  htmlFor='dashboard-referral-link'
                  className='text-xs font-medium'
                >
                  {t('Referral link')}
                </label>
                <div className='flex min-w-0 items-center gap-2'>
                  <Input
                    id='dashboard-referral-link'
                    value={affiliateLink}
                    readOnly
                    className='min-w-0 font-mono'
                  />
                  <CopyButton
                    value={affiliateLink}
                    variant='outline'
                    tooltip={t('Copy referral link')}
                    aria-label={t('Copy referral link')}
                  />
                </div>
              </div>
            </div>
          </CardContent>
        </Card>
      </CardStaggerItem>
    </CardStaggerContainer>
  )
}
