import { api } from '@/lib/api'

import type {
  ApiResponse,
  InvoiceApplication,
  InvoiceApplicationDetail,
  InvoiceEmailTemplate,
  InvoiceOrder,
  InvoiceStatus,
  InvoiceTitle,
  InvoiceTitleInput,
  PageData,
} from './types'

export async function getInvoiceTitles(): Promise<
  ApiResponse<{ items: InvoiceTitle[] }>
> {
  const response = await api.get('/api/user/invoice/titles')
  return response.data
}

export async function createInvoiceTitle(
  input: InvoiceTitleInput
): Promise<ApiResponse<InvoiceTitle>> {
  const response = await api.post('/api/user/invoice/titles', input)
  return response.data
}

export async function getInvoiceOrders(
  status: InvoiceStatus | 'all'
): Promise<ApiResponse<PageData<InvoiceOrder>>> {
  const response = await api.get('/api/user/invoice/orders', {
    params: { status, p: 1, page_size: 100 },
  })
  return response.data
}

export async function updateInvoiceTitle(
  id: number,
  input: InvoiceTitleInput
): Promise<ApiResponse<InvoiceTitle>> {
  const response = await api.put(`/api/user/invoice/titles/${id}`, input)
  return response.data
}

export async function deleteInvoiceTitle(id: number): Promise<ApiResponse> {
  const response = await api.delete(`/api/user/invoice/titles/${id}`)
  return response.data
}

export async function getInvoiceApplications(
  status: InvoiceStatus | 'all',
  admin = false,
  page = 1,
  pageSize = 100,
  keyword = ''
): Promise<ApiResponse<PageData<InvoiceApplication>>> {
  const response = await api.get(
    admin ? '/api/user/invoice' : '/api/user/invoice/applications',
    {
      params: {
        status,
        p: page,
        page_size: pageSize,
        ...(keyword.trim() ? { keyword: keyword.trim() } : {}),
      },
    }
  )
  return response.data
}

export async function createInvoiceApplication(
  titleId: number,
  topupIds: number[]
): Promise<ApiResponse<InvoiceApplication>> {
  const response = await api.post('/api/user/invoice/applications', {
    title_id: titleId,
    topup_ids: topupIds,
  })
  return response.data
}

export async function getInvoiceApplicationDetail(
  id: number,
  admin: boolean
): Promise<ApiResponse<InvoiceApplicationDetail>> {
  const response = await api.get(
    admin ? `/api/user/invoice/${id}` : `/api/user/invoice/applications/${id}`
  )
  return response.data
}

export async function getInvoiceEmailTemplate(): Promise<
  ApiResponse<InvoiceEmailTemplate>
> {
  const response = await api.get('/api/user/invoice/template')
  return response.data
}

export async function updateInvoiceEmailTemplate(
  input: InvoiceEmailTemplate
): Promise<ApiResponse<InvoiceEmailTemplate>> {
  const response = await api.put('/api/user/invoice/template', input)
  return response.data
}

export async function issueInvoice(
  id: number,
  file: File
): Promise<ApiResponse> {
  const formData = new FormData()
  formData.append('file', file)
  const response = await api.post(`/api/user/invoice/${id}/issue`, formData)
  return response.data
}
