export type InvoiceStatus = 'unissued' | 'issuing' | 'issued'

export interface ApiResponse<T = unknown> {
  success: boolean
  message?: string
  data?: T
}

export interface PageData<T> {
  items: T[]
  total: number
  page: number
  page_size: number
}

export interface InvoiceTitle {
  id: number
  user_id: number
  name: string
  tax_number: string
  address: string
  email: string
}

export interface InvoiceTitleInput {
  name: string
  tax_number: string
  address: string
  email: string
}

export interface InvoiceOrder {
  id: number
  user_id: number
  amount: number
  money: number
  trade_no: string
  payment_method: string
  complete_time: number
  invoice_status: InvoiceStatus
}

export interface InvoiceApplication {
  id: number
  user_id: number
  status: InvoiceStatus
  total_money: number
  recipient_email: string
  title_name: string
  title_tax_number: string
  title_address: string
  last_error: string
  created_at: number
  issued_at: number
  pdf_file_name: string
  pdf_sha256?: string
  pdf_size?: number
}

export interface InvoiceApplicationOrder {
  id: number
  topup_id: number
  trade_no: string
  money: number
  amount: number
}

export interface InvoiceApplicationDetail {
  application: InvoiceApplication
  orders: InvoiceApplicationOrder[]
}

export interface InvoiceEmailTemplate {
  subject: string
  body: string
}
