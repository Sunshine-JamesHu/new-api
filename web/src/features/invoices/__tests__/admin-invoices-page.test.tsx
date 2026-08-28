import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { fireEvent, render, screen, waitFor } from '@testing-library/react'
import i18n from 'i18next'
import { I18nextProvider } from 'react-i18next'
import { beforeEach, describe, expect, test, vi } from 'vitest'

const {
  getInvoiceApplications,
  getInvoiceApplicationDetail,
  getInvoiceEmailTemplate,
  issueInvoice,
  updateInvoiceEmailTemplate,
} = vi.hoisted(() => ({
  getInvoiceApplications: vi.fn(),
  getInvoiceApplicationDetail: vi.fn(),
  getInvoiceEmailTemplate: vi.fn(),
  issueInvoice: vi.fn(),
  updateInvoiceEmailTemplate: vi.fn(),
}))

vi.mock('../api', () => ({
  getInvoiceApplications,
  getInvoiceApplicationDetail,
  getInvoiceEmailTemplate,
  issueInvoice,
  updateInvoiceEmailTemplate,
}))

const { AdminInvoicesPage } = await import('../admin-invoices-page')

beforeEach(() => {
  getInvoiceApplications.mockResolvedValue({
    success: true,
    data: {
      items: [
        {
          id: 7,
          status: 'issuing',
          total_money: 12.5,
          title_name: 'Company',
          recipient_email: 'invoice@example.com',
          last_error: '',
          created_at: 1,
          issued_at: 0,
          pdf_file_name: '',
        },
      ],
      total: 1,
    },
  })
  getInvoiceApplicationDetail.mockResolvedValue({
    success: true,
    data: {
      application: {
        id: 7,
        status: 'issuing',
        total_money: 12.5,
        title_name: 'Company',
        title_tax_number: 'TAX',
        title_address: 'Address',
        recipient_email: 'invoice@example.com',
        last_error: '',
        pdf_file_name: '',
        pdf_size: 0,
      },
      orders: [],
    },
  })
  getInvoiceEmailTemplate.mockResolvedValue({
    success: true,
    data: { subject: 'Subject', body: 'Body' },
  })
  issueInvoice.mockReset()
  issueInvoice.mockResolvedValue({ success: true })
  updateInvoiceEmailTemplate.mockReset()
  updateInvoiceEmailTemplate.mockResolvedValue({ success: true })
})

function renderPage() {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  })
  return render(
    <QueryClientProvider client={queryClient}>
      <I18nextProvider i18n={i18n}>
        <AdminInvoicesPage />
      </I18nextProvider>
    </QueryClientProvider>
  )
}

describe('admin invoice page', () => {
  test('shows issuing request and rejects a non-PDF file before upload', async () => {
    renderPage()
    expect(await screen.findByText('Company')).toBeVisible()
    fireEvent.click(screen.getByRole('button', { name: /Issue invoice/ }))
    const input = document.querySelector('input[type="file"]')
    expect(input).toHaveAttribute('accept', 'application/pdf,.pdf')
    fireEvent.change(input as HTMLInputElement, {
      target: {
        files: [new File(['not pdf'], 'invoice.txt', { type: 'text/plain' })],
      },
    })
    expect(issueInvoice).not.toHaveBeenCalled()
    expect(screen.getByText('Drop a PDF here or choose a file')).toBeVisible()
  })

  test('filters requests by keyword', async () => {
    renderPage()
    const search = await screen.findByRole('textbox', { name: 'Search' })
    fireEvent.change(search, { target: { value: 'invoice@example.com' } })
    await waitFor(() =>
      expect(getInvoiceApplications).toHaveBeenLastCalledWith(
        'issuing',
        true,
        1,
        100,
        'invoice@example.com'
      )
    )
  })

  test('loads invoice details including the translated tax number label', async () => {
    renderPage()
    fireEvent.click(await screen.findByRole('button', { name: 'Details' }))
    expect(await screen.findByRole('heading', { name: 'Invoice details' })).toBeVisible()
    const taxValue = await screen.findByText('TAX')
    expect(taxValue.parentElement).toHaveTextContent(/Tax number|单位税号/)
    expect(getInvoiceApplicationDetail).toHaveBeenCalledWith(7, true)
  })

  test('saves the invoice email template', async () => {
    renderPage()
    fireEvent.click(await screen.findByRole('button', { name: 'Email template' }))
    fireEvent.change(screen.getByLabelText('Email subject'), {
      target: { value: 'Updated subject' },
    })
    fireEvent.change(screen.getByLabelText('Email body'), {
      target: { value: 'Updated body' },
    })
    fireEvent.click(screen.getByRole('button', { name: 'Save' }))
    await waitFor(() =>
      expect(updateInvoiceEmailTemplate).toHaveBeenCalledWith({
        subject: 'Updated subject',
        body: 'Updated body',
      })
    )
  })

  test('uploads a valid PDF and sends the invoice', async () => {
    renderPage()
    fireEvent.click(await screen.findByRole('button', { name: /Issue invoice/ }))
    const input = document.querySelector('input[type="file"]')
    fireEvent.change(input as HTMLInputElement, {
      target: {
        files: [new File(['%PDF-1.7 test'], 'invoice.pdf', { type: 'application/pdf' })],
      },
    })
    fireEvent.click(screen.getByRole('button', { name: 'Send invoice' }))
    await waitFor(() => expect(issueInvoice).toHaveBeenCalledWith(7, expect.any(File)))
  })
})
