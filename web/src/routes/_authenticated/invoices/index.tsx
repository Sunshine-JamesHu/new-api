import { createFileRoute } from '@tanstack/react-router'

import { UserInvoicesPage } from '@/features/invoices'

export const Route = createFileRoute('/_authenticated/invoices/')({
  component: UserInvoicesPage,
})
