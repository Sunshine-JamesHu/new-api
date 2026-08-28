import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { fireEvent, render, screen, waitFor } from '@testing-library/react'
import i18n from 'i18next'
import { I18nextProvider } from 'react-i18next'
import { beforeEach, describe, expect, test, vi } from 'vitest'

const {
  getInvoiceTitles,
  getInvoiceOrders,
  getInvoiceApplications,
  createInvoiceApplication,
  deleteInvoiceTitle,
} = vi.hoisted(() => ({
  getInvoiceTitles: vi.fn(),
  getInvoiceOrders: vi.fn(),
  getInvoiceApplications: vi.fn(),
  createInvoiceApplication: vi.fn(),
  deleteInvoiceTitle: vi.fn(),
}))

vi.mock('../api', () => ({
  getInvoiceTitles,
  getInvoiceOrders,
  getInvoiceApplications,
  createInvoiceApplication,
  deleteInvoiceTitle,
}))

const { UserInvoicesPage } = await import('../user-invoices-page')

beforeEach(() => {
  getInvoiceTitles.mockResolvedValue({
    success: true,
    data: {
      items: [
        {
          id: 3,
          name: 'Company',
          tax_number: 'TAX',
          address: 'Address',
          email: 'invoice@example.com',
        },
      ],
    },
  })
  getInvoiceOrders.mockResolvedValue({
    success: true,
    data: {
      items: [
        {
          id: 11,
          trade_no: 'order-11',
          money: 10,
          amount: 100,
          invoice_status: 'unissued',
          status: 'success',
          complete_time: 1,
        },
      ],
      total: 1,
    },
  })
  getInvoiceApplications.mockResolvedValue({
    success: true,
    data: { items: [], total: 0 },
  })
  createInvoiceApplication.mockResolvedValue({
    success: true,
    data: { id: 20 },
  })
  deleteInvoiceTitle.mockResolvedValue({ success: true })
})

function renderPage() {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  })
  return render(
    <QueryClientProvider client={queryClient}>
      <I18nextProvider i18n={i18n}>
        <UserInvoicesPage />
      </I18nextProvider>
    </QueryClientProvider>
  )
}

function clickCheckbox(label: string) {
  const checkbox = screen
    .getByText(label)
    .closest('label')
    ?.querySelector('[data-slot="checkbox"]')
  expect(checkbox).toBeTruthy()
  fireEvent.click(checkbox as HTMLElement)
}

describe('user invoice page', () => {
  test('opens invoice title management with existing titles and add action', async () => {
    renderPage()
    fireEvent.click(
      await screen.findByRole('button', { name: 'Invoice title management' })
    )

    expect(
      await screen.findByRole('heading', { name: 'Invoice title management' })
    ).toBeVisible()
    expect(screen.getByText('Company')).toBeVisible()
    expect(
      screen.getByRole('button', { name: 'Add invoice title' })
    ).toBeVisible()
  })

  test('opens add and edit title dialogs from title management', async () => {
    renderPage()
    fireEvent.click(
      await screen.findByRole('button', { name: 'Invoice title management' })
    )
    fireEvent.click(screen.getByRole('button', { name: 'Add invoice title' }))
    expect(
      await screen.findByRole('heading', { name: 'Add invoice title' })
    ).toBeVisible()
  })

  test('deletes a saved title from title management', async () => {
    renderPage()
    fireEvent.click(
      await screen.findByRole('button', { name: 'Invoice title management' })
    )
    fireEvent.click(
      screen.getByRole('button', { name: 'Delete invoice title' })
    )
    await waitFor(() => expect(deleteInvoiceTitle).toHaveBeenCalledWith(3))
  })

  test('guides users to add a title before requesting an invoice', async () => {
    getInvoiceTitles.mockResolvedValueOnce({
      success: true,
      data: { items: [] },
    })
    renderPage()
    await screen.findByText('order-11')
    clickCheckbox('order-11')
    fireEvent.click(screen.getByRole('button', { name: /Request invoice/ }))
    expect(
      await screen.findByRole('heading', { name: 'Add invoice title' })
    ).toBeVisible()
  })

  test('selects all available orders and shows the total before confirmation', async () => {
    renderPage()
    expect(await screen.findByText('order-11')).toBeVisible()
    clickCheckbox('Select all available orders')
    expect(screen.getByText(/1 orders selected/)).toBeVisible()
    expect(screen.getAllByText(/10/).length).toBeGreaterThan(0)
    fireEvent.click(screen.getByRole('button', { name: /Request invoice/ }))
    expect(
      await screen.findByRole('heading', { name: 'Confirm invoice request' })
    ).toBeVisible()
    expect(screen.getAllByText(/10/).length).toBeGreaterThan(0)
  })

  test('submits selected orders with the chosen title', async () => {
    renderPage()
    await screen.findByText('order-11')
    clickCheckbox('order-11')
    fireEvent.click(screen.getByRole('button', { name: /Request invoice/ }))
    fireEvent.click(await screen.findByRole('button', { name: 'Confirm' }))
    await waitFor(() =>
      expect(createInvoiceApplication).toHaveBeenCalledWith(3, [11])
    )
  })

  test('shows complete invoice titles in a radio list and submits the selected title', async () => {
    getInvoiceTitles.mockResolvedValueOnce({
      success: true,
      data: {
        items: [
          {
            id: 3,
            name: 'Company',
            tax_number: 'TAX',
            address: 'Address',
            email: 'invoice@example.com',
          },
          {
            id: 4,
            name: 'Second Company',
            tax_number: 'SECOND-TAX',
            address: 'Second registered address',
            email: 'second@example.com',
          },
        ],
      },
    })
    renderPage()
    await screen.findByText('order-11')
    clickCheckbox('order-11')
    fireEvent.click(screen.getByRole('button', { name: /Request invoice/ }))

    const titleOptions = await screen.findAllByRole('radio')
    expect(titleOptions).toHaveLength(2)
    expect(titleOptions[0]).toBeChecked()
    expect(screen.queryByRole('combobox')).not.toBeInTheDocument()
    expect(screen.getByText(/SECOND-TAX/)).toBeVisible()
    expect(screen.getByText(/Second registered address/)).toBeVisible()
    expect(screen.getByText(/second@example.com/)).toBeVisible()

    fireEvent.click(titleOptions[1])
    fireEvent.click(screen.getByRole('button', { name: 'Confirm' }))
    await waitFor(() =>
      expect(createInvoiceApplication).toHaveBeenCalledWith(4, [11])
    )
  })
})
