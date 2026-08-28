import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { fireEvent, render, screen, waitFor } from '@testing-library/react'
import i18n from 'i18next'
import { I18nextProvider } from 'react-i18next'
import { beforeEach, describe, expect, test, vi } from 'vitest'

const { createInvoiceTitle, updateInvoiceTitle } = vi.hoisted(() => ({
  createInvoiceTitle: vi.fn(),
  updateInvoiceTitle: vi.fn(),
}))

vi.mock('../api', () => ({ createInvoiceTitle, updateInvoiceTitle }))

const { InvoiceTitleDialog } = await import('../invoice-title-dialog')

function renderDialog() {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  })
  return render(
    <QueryClientProvider client={queryClient}>
      <I18nextProvider i18n={i18n}>
        <InvoiceTitleDialog open onOpenChange={() => undefined} />
      </I18nextProvider>
    </QueryClientProvider>
  )
}

beforeEach(() => {
  createInvoiceTitle.mockReset()
  updateInvoiceTitle.mockReset()
  createInvoiceTitle.mockResolvedValue({ success: true, data: {} })
})

describe('invoice title dialog', () => {
  test('uses instructional placeholders instead of customer data', () => {
    renderDialog()

    expect(screen.getByLabelText('Company name')).toHaveAttribute(
      'placeholder',
      'Enter the legal company name'
    )
    expect(screen.getByLabelText('Tax number')).toHaveAttribute(
      'placeholder',
      'Enter the company tax identification number'
    )
    expect(screen.getByLabelText('Registered address')).toHaveAttribute(
      'placeholder',
      'Enter the registered company address'
    )
    expect(screen.getByLabelText('Invoice email')).toHaveAttribute(
      'placeholder',
      'Enter the email address for invoice delivery'
    )
  })

  test('keeps save disabled until all required fields are valid', () => {
    renderDialog()
    const save = screen.getByRole('button', { name: 'Save' })
    expect(save).toBeDisabled()

    fireEvent.change(screen.getByLabelText('Company name'), {
      target: { value: 'Company' },
    })
    fireEvent.change(screen.getByLabelText('Tax number'), {
      target: { value: 'TAX' },
    })
    fireEvent.change(screen.getByLabelText('Registered address'), {
      target: { value: 'Address' },
    })
    fireEvent.change(screen.getByLabelText('Invoice email'), {
      target: { value: 'invoice@example.com' },
    })
    expect(save).toBeEnabled()
  })

  test('submits normalized title fields and closes after a successful response', async () => {
    const onOpenChange = vi.fn()
    const queryClient = new QueryClient({
      defaultOptions: { queries: { retry: false } },
    })
    render(
      <QueryClientProvider client={queryClient}>
        <I18nextProvider i18n={i18n}>
          <InvoiceTitleDialog open onOpenChange={onOpenChange} />
        </I18nextProvider>
      </QueryClientProvider>
    )
    fireEvent.change(screen.getByLabelText('Company name'), {
      target: { value: '  Company  ' },
    })
    fireEvent.change(screen.getByLabelText('Tax number'), {
      target: { value: ' TAX ' },
    })
    fireEvent.change(screen.getByLabelText('Registered address'), {
      target: { value: ' Address ' },
    })
    fireEvent.change(screen.getByLabelText('Invoice email'), {
      target: { value: 'invoice@example.com' },
    })
    fireEvent.click(screen.getByRole('button', { name: 'Save' }))

    await waitFor(() =>
      expect(createInvoiceTitle).toHaveBeenCalledWith({
        name: '  Company  ',
        tax_number: ' TAX ',
        address: ' Address ',
        email: 'invoice@example.com',
      })
    )
    await waitFor(() => expect(onOpenChange).toHaveBeenCalledWith(false))
  })
})
