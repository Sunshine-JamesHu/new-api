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
import i18next from 'i18next'
import { useState, useCallback } from 'react'
import { toast } from 'sonner'

import {
  calculateAmount,
  calculateAlipayAmount,
  calculateStripeAmount,
  calculateWaffoAmount,
  calculateWaffoPancakeAmount,
  requestPayment,
  requestAlipayPayment,
  requestStripePayment,
  isApiSuccess,
} from '../api'
import {
  isStripePayment,
  isOfficialAlipayPayment,
  isWaffoPayment,
  isWaffoPancakePayment,
  submitPaymentForm,
} from '../lib'
import type { AmountRequest, AmountResponse } from '../types'

// ============================================================================
// Payment Hook
// ============================================================================

type AmountCalculator = (request: AmountRequest) => Promise<AmountResponse>

export interface PaymentAmountCalculators {
  regular: AmountCalculator
  stripe: AmountCalculator
  waffo: AmountCalculator
  waffoPancake: AmountCalculator
}

const defaultPaymentAmountCalculators: PaymentAmountCalculators = {
  regular: calculateAmount,
  stripe: calculateStripeAmount,
  waffo: calculateWaffoAmount,
  waffoPancake: calculateWaffoPancakeAmount,
}

export async function requestPaymentAmount(
  topupAmount: number,
  paymentType: string,
  calculators: PaymentAmountCalculators = defaultPaymentAmountCalculators
): Promise<number> {
  let calculator = calculators.regular
  if (isStripePayment(paymentType)) {
    calculator = calculators.stripe
  } else if (isWaffoPayment(paymentType)) {
    calculator = calculators.waffo
  } else if (isWaffoPancakePayment(paymentType)) {
    calculator = calculators.waffoPancake
  }

  const response = await calculator({ amount: topupAmount })
  if (!isApiSuccess(response) || !response.data) {
    return 0
  }

  return Number.parseFloat(response.data)
}

export function usePayment() {
  const [amount, setAmount] = useState<number>(0)
  const [calculating, setCalculating] = useState(false)
  const [processing, setProcessing] = useState(false)
  const [alipayQrCode, setAlipayQrCode] = useState('')

  const calculateAmountForMethod = useCallback(
    async (topupAmount: number, paymentType: string, provider?: string) => {
      if (isStripePayment(paymentType)) {
        return calculateStripeAmount({ amount: topupAmount })
      }
      if (isOfficialAlipayPayment(paymentType, provider)) {
        return calculateAlipayAmount({ amount: topupAmount })
      }
      if (isWaffoPayment(paymentType)) {
        return calculateWaffoAmount({ amount: topupAmount })
      }
      if (isWaffoPancakePayment(paymentType)) {
        return calculateWaffoPancakeAmount({ amount: topupAmount })
      }
      return calculateAmount({ amount: topupAmount })
    },
    []
  )

  // Calculate payment amount
  const calculatePaymentAmount = useCallback(
    async (topupAmount: number, paymentType: string, provider?: string) => {
      try {
        setCalculating(true)
        const response = await calculateAmountForMethod(
          topupAmount,
          paymentType,
          provider
        )

        if (isApiSuccess(response) && response.data) {
          const calculatedAmount = Number.parseFloat(response.data)
          setAmount(calculatedAmount)
          return calculatedAmount
        }

        // Don't show error for calculation, just set to 0
        setAmount(0)
        return 0
      } catch {
        setAmount(0)
        return 0
      } finally {
        setCalculating(false)
      }
    },
    [calculateAmountForMethod]
  )

  // Process payment
  const processPayment = useCallback(
    async (topupAmount: number, paymentType: string, provider?: string) => {
      let payWindow: Window | null = null
      let keepPayWindowOpen = false

      try {
        setProcessing(true)

        const isStripe = isStripePayment(paymentType)
        const isAlipay = isOfficialAlipayPayment(paymentType, provider)
        const amount = Math.floor(topupAmount)

        if (isStripe) {
          const response = await requestStripePayment({
            amount,
            payment_method: 'stripe',
          })

          if (!isApiSuccess(response)) {
            toast.error(response.message || i18next.t('Payment request failed'))
            return false
          }
          if (response.data?.pay_link) {
            window.open(response.data.pay_link, '_blank')
            toast.success(i18next.t('Redirecting to payment page...'))
            return true
          }
          return false
        }

        if (isAlipay) {
          payWindow = window.open('', '_blank')

          const response = await requestAlipayPayment({
            amount,
            payment_method: paymentType,
            is_mobile: /Android|iPhone|iPad|iPod|Mobile/i.test(
              navigator.userAgent
            ),
          })

          if (!isApiSuccess(response)) {
            payWindow?.close()
            toast.error(response.message || i18next.t('Payment request failed'))
            return false
          }
          if (response.data) {
            if (response.data.pay_url) {
              if (payWindow) {
                payWindow.location.href = response.data.pay_url
                keepPayWindowOpen = true
              } else {
                window.location.href = response.data.pay_url
              }
              toast.success(i18next.t('Redirecting to payment page...'))
              return true
            }
            if (response.data.qr_code) {
              payWindow?.close()
              setAlipayQrCode(response.data.qr_code)
              toast.success(i18next.t('Alipay QR code opened'))
              return true
            }
          }
          if (!keepPayWindowOpen) {
            payWindow?.close()
          }
          return false
        }

        const response = await requestPayment({
          amount,
          payment_method: paymentType,
        })
        if (!isApiSuccess(response)) {
          toast.error(response.message || i18next.t('Payment request failed'))
          return false
        }
        if (response.data) {
          const url = (response as unknown as { url?: string }).url
          if (url) {
            submitPaymentForm(url, response.data)
            toast.success(i18next.t('Redirecting to payment page...'))
            return true
          }
        }

        return false
      } catch {
        toast.error(i18next.t('Payment request failed'))
        return false
      } finally {
        if (!keepPayWindowOpen) {
          payWindow?.close()
        }
        setProcessing(false)
      }
    },
    []
  )

  return {
    amount,
    calculating,
    processing,
    alipayQrCode,
    calculatePaymentAmount,
    processPayment,
    setAmount,
    setAlipayQrCode,
  }
}
