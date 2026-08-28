# Invoice Local Test

This document records the local checks for the invoice self-service and administrator workflow.

## Start

Use SQLite for a disposable local database and start the backend from the repository root:

```powershell
$env:SQL_DSN = 'sqlite://.tmp/invoice-runtime.db'
go run .
```

Start the frontend in a second terminal:

```powershell
cd web
bun run dev
```

Open the printed frontend URL. The payment settings route must render both payment text areas without a `Textarea is not defined` browser error.

## Workflow checks

1. Sign in as a user, open **Invoices**, create a title with name, tax number, address, and email, then select one or more successful top-up orders.
2. Confirm the dialog total equals the sum returned by `GET /api/user/invoice/orders`; submit and verify the orders and application become `issuing`.
3. Sign in as an administrator, open **Invoice management**, inspect the application details, and verify the title snapshot and order list.
4. Configure the existing SMTP settings, upload a real PDF no larger than 10 MiB, and issue the invoice. Verify the message contains the PDF attachment and that the application and linked orders become `issued` only after SMTP accepts the message.
5. Force SMTP failure and verify the application and linked orders remain `issuing`, the error is visible, and a later retry succeeds.
6. Verify an already `issued` application cannot be sent twice, and legacy `invoice_issued=false/true` records remain `unissued/issued`.

## Automated verification

```powershell
go test ./common ./model ./controller ./router
cd web
bun run test
bun run typecheck
bun run build
```

The full repository test suite may include unrelated pre-existing task-settlement failures; report those separately from invoice test results.
