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
import { useState, useEffect, useCallback } from 'react'
import { toast } from 'sonner'

import { useIsAdmin } from '@/hooks/use-admin'

import {
  getUserBillingHistory,
  getAllBillingHistory,
  completeOrder,
  updateTopUpInvoice,
  isApiSuccess,
} from '../api'
import type { InvoiceStatusFilter, TopupRecord } from '../types'

// ============================================================================
// Billing History Hook
// ============================================================================

interface UseBillingHistoryOptions {
  /** Initial page number */
  initialPage?: number
  /** Initial page size */
  initialPageSize?: number
}

export function useBillingHistory(options: UseBillingHistoryOptions = {}) {
  const { initialPage = 1, initialPageSize = 10 } = options
  const isAdmin = useIsAdmin()

  const [records, setRecords] = useState<TopupRecord[]>([])
  const [total, setTotal] = useState(0)
  const [page, setPage] = useState(initialPage)
  const [pageSize, setPageSize] = useState(initialPageSize)
  const [keyword, setKeyword] = useState('')
  const [userId, setUserId] = useState('')
  const [invoiceStatus, setInvoiceStatus] = useState<InvoiceStatusFilter>('all')
  const [loading, setLoading] = useState(false)
  const [completing, setCompleting] = useState(false)
  const [updatingInvoice, setUpdatingInvoice] = useState(false)

  /**
   * Fetch billing history
   */
  const fetchBillingHistory = useCallback(async () => {
    setLoading(true)
    try {
      const parsedUserId = Number(userId)
      const adminUserId = userId === '' ? undefined : parsedUserId
      const response = isAdmin
        ? await getAllBillingHistory(
            page,
            pageSize,
            keyword,
            invoiceStatus,
            adminUserId
          )
        : await getUserBillingHistory(page, pageSize, keyword, invoiceStatus)

      if (isApiSuccess(response) && response.data) {
        setRecords(response.data.items || [])
        setTotal(response.data.total || 0)
      } else {
        toast.error(
          response.message || i18next.t('Failed to load billing history')
        )
        setRecords([])
        setTotal(0)
      }
    } catch (error) {
      // eslint-disable-next-line no-console
      console.error('Failed to fetch billing history:', error)
      toast.error(i18next.t('Failed to load billing history'))
      setRecords([])
      setTotal(0)
    } finally {
      setLoading(false)
    }
  }, [invoiceStatus, isAdmin, keyword, page, pageSize, userId])

  /**
   * Complete a pending order (admin only)
   */
  const handleCompleteOrder = useCallback(
    async (tradeNo: string) => {
      if (!isAdmin) {
        toast.error(i18next.t('Admin access required'))
        return false
      }

      setCompleting(true)
      try {
        const response = await completeOrder({ trade_no: tradeNo })
        if (isApiSuccess(response)) {
          toast.success(i18next.t('Order completed successfully'))
          // Refresh the list
          await fetchBillingHistory()
          return true
        } else {
          toast.error(response.message || i18next.t('Failed to complete order'))
          return false
        }
      } catch (error) {
        // eslint-disable-next-line no-console
        console.error('Failed to complete order:', error)
        toast.error(i18next.t('Failed to complete order'))
        return false
      } finally {
        setCompleting(false)
      }
    },
    [isAdmin, fetchBillingHistory]
  )

  /**
   * Change page
   */
  const handlePageChange = useCallback((newPage: number) => {
    setPage(newPage)
  }, [])

  /**
   * Change page size
   */
  const handlePageSizeChange = useCallback((newPageSize: number) => {
    setPageSize(newPageSize)
    setPage(1) // Reset to first page when changing page size
  }, [])

  /**
   * Search by keyword
   */
  const handleSearch = useCallback((newKeyword: string) => {
    setKeyword(newKeyword)
    setPage(1) // Reset to first page when searching
  }, [])

  const handleUserIdSearch = useCallback((newUserId: string) => {
    setUserId(newUserId)
    setPage(1)
  }, [])

  const handleInvoiceStatusChange = useCallback(
    (newInvoiceStatus: InvoiceStatusFilter) => {
      setInvoiceStatus(newInvoiceStatus)
      setPage(1)
    },
    []
  )

  const handleUpdateInvoiceStatus = useCallback(
    async (tradeNo: string, invoiceIssued: boolean) => {
      if (!isAdmin) {
        toast.error(i18next.t('Admin access required'))
        return false
      }

      setUpdatingInvoice(true)
      try {
        const response = await updateTopUpInvoice({
          trade_no: tradeNo,
          invoice_issued: invoiceIssued,
        })
        if (isApiSuccess(response)) {
          toast.success(
            i18next.t(
              invoiceIssued
                ? 'Order marked as invoiced'
                : 'Order marked as not invoiced'
            )
          )
          await fetchBillingHistory()
          return true
        }
        toast.error(
          response.message || i18next.t('Failed to update invoice status')
        )
        return false
      } catch (error) {
        // eslint-disable-next-line no-console
        console.error('Failed to update invoice status:', error)
        toast.error(i18next.t('Failed to update invoice status'))
        return false
      } finally {
        setUpdatingInvoice(false)
      }
    },
    [fetchBillingHistory, isAdmin]
  )

  // Fetch data when dependencies change
  useEffect(() => {
    fetchBillingHistory()
  }, [fetchBillingHistory])

  return {
    records,
    total,
    page,
    pageSize,
    keyword,
    userId,
    invoiceStatus,
    loading,
    completing,
    updatingInvoice,
    isAdmin,
    handlePageChange,
    handlePageSizeChange,
    handleSearch,
    handleUserIdSearch,
    handleInvoiceStatusChange,
    handleCompleteOrder,
    handleUpdateInvoiceStatus,
    refresh: fetchBillingHistory,
  }
}
