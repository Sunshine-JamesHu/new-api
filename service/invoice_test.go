package service

import (
	"bufio"
	"bytes"
	"mime/multipart"
	"net"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func startInvoiceSMTP(t *testing.T) (string, int, <-chan string) {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	host, portText, err := net.SplitHostPort(listener.Addr().String())
	require.NoError(t, err)
	port, err := strconv.Atoi(portText)
	require.NoError(t, err)
	messages := make(chan string, 1)
	go func() {
		defer listener.Close()
		conn, err := listener.Accept()
		if err != nil {
			return
		}
		defer conn.Close()
		r := bufio.NewReader(conn)
		w := bufio.NewWriter(conn)
		write := func(value string) bool {
			if _, err := w.WriteString(value + "\r\n"); err != nil {
				return false
			}
			return w.Flush() == nil
		}
		if !write("220 local invoice smtp") {
			return
		}
		for {
			line, err := r.ReadString('\n')
			if err != nil {
				return
			}
			switch {
			case strings.HasPrefix(strings.ToUpper(line), "EHLO"), strings.HasPrefix(strings.ToUpper(line), "HELO"):
				if !write("250-local invoice smtp") || !write("250 8BITMIME") {
					return
				}
			case strings.HasPrefix(strings.ToUpper(line), "MAIL FROM"), strings.HasPrefix(strings.ToUpper(line), "RCPT TO"):
				if !write("250 OK") {
					return
				}
			case strings.EqualFold(strings.TrimSpace(line), "DATA"):
				if !write("354 End data with <CR><LF>.<CR><LF>") {
					return
				}
				var message strings.Builder
				for {
					dataLine, readErr := r.ReadString('\n')
					if readErr != nil {
						return
					}
					if strings.TrimRight(dataLine, "\r\n") == "." {
						break
					}
					message.WriteString(dataLine)
				}
				messages <- message.String()
				if !write("250 queued") {
					return
				}
			case strings.EqualFold(strings.TrimSpace(line), "QUIT"):
				write("221 bye")
				return
			default:
				if !write("250 OK") {
					return
				}
			}
		}
	}()
	t.Cleanup(func() { listener.Close() })
	return host, port, messages
}

func invoiceFileHeader(t *testing.T, name string, content []byte) *multipart.FileHeader {
	t.Helper()
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile("file", name)
	require.NoError(t, err)
	_, err = part.Write(content)
	require.NoError(t, err)
	require.NoError(t, writer.Close())
	req := httptest.NewRequest("POST", "/", &body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	require.NoError(t, req.ParseMultipartForm(int64(len(content))+1024))
	return req.MultipartForm.File["file"][0]
}

func TestValidateInvoicePDFRequiresSignatureAndSanitizesFilename(t *testing.T) {
	assert.Equal(t, "invoice.pdf", sanitizeInvoiceFilename("C:\\tmp\\invoice\"\r\n.pdf"))
	_, _, _, err := ValidateInvoicePDF(nil)
	assert.EqualError(t, err, ErrInvoicePDFInvalid.Error())
	assert.Equal(t, "invoice.pdf", sanitizeInvoiceFilename("invoice.pdf"))
}

func TestValidateInvoicePDFRejectsFakeAndOversizedFiles(t *testing.T) {
	_, _, _, err := ValidateInvoicePDF(invoiceFileHeader(t, "invoice.pdf", []byte("not a pdf")))
	require.ErrorIs(t, err, ErrInvoicePDFInvalid)
	tooLarge := append([]byte("%PDF-1.7"), bytes.Repeat([]byte("x"), InvoiceMaxPDFSize)...)
	_, _, _, err = ValidateInvoicePDF(invoiceFileHeader(t, "invoice.pdf", tooLarge))
	require.ErrorIs(t, err, ErrInvoicePDFInvalid)
	data, name, size, err := ValidateInvoicePDF(invoiceFileHeader(t, "nested\\invoice.pdf", []byte("%PDF-1.7 body")))
	require.NoError(t, err)
	assert.Equal(t, "invoice.pdf", name)
	assert.Equal(t, int64(len(data)), size)
}

func TestRenderInvoiceTemplateUsesLiteralReplacementAndHTMLEscaping(t *testing.T) {
	application := &model.InvoiceApplication{
		ID:             42,
		RecipientEmail: "user@example.com",
		TitleName:      `<script>alert(1)</script>`,
		TotalMoney:     12.5,
	}
	orders := []model.InvoiceApplicationOrder{{TradeNo: `<img src=x>`, Money: 12.5}}

	subject, body, err := RenderInvoiceTemplate(InvoiceTemplate{
		Subject: "Invoice {{invoice_id}} {{unknown}}",
		Body:    "{{title_name}} {{orders}}",
	}, application, orders)
	require.NoError(t, err)
	assert.Equal(t, "Invoice 42 {{unknown}}", subject)
	assert.NotContains(t, body, "<script>")
	assert.NotContains(t, body, "<img")
	assert.Contains(t, body, "&lt;script&gt;")
}

func TestRenderInvoiceTemplateUsesFormalChineseDefaults(t *testing.T) {
	application := &model.InvoiceApplication{
		ID:         43,
		TitleName:  "测试单位",
		TotalMoney: 88.8,
	}
	orders := []model.InvoiceApplicationOrder{{TradeNo: "ORDER-001", Money: 88.8}}

	subject, body, err := RenderInvoiceTemplate(InvoiceTemplate{}, application, orders)
	require.NoError(t, err)
	assert.Equal(t, "发票开具通知（43）", subject)
	assert.Contains(t, body, "尊敬的 测试单位：")
	assert.Contains(t, body, "发票金额为 88.80")
	assert.Contains(t, body, "关联订单：")
	assert.Contains(t, body, "ORDER-001 (88.80)")
	assert.Contains(t, body, common.SystemName)
}

func TestSaveInvoiceTemplateRejectsHeaderInjectionAndOversizedValues(t *testing.T) {
	assert.Error(t, SaveInvoiceTemplate(InvoiceTemplate{Subject: "ok\r\nBcc: attacker@example.com", Body: "body"}))
	assert.Error(t, SaveInvoiceTemplate(InvoiceTemplate{Subject: "ok", Body: strings.Repeat("x", 20001)}))
}

func TestIssueInvoiceEndToEndSendsPDFBeforeMarkingOrdersIssued(t *testing.T) {
	originalServer, originalPort := common.SMTPServer, common.SMTPPort
	originalAccount, originalFrom, originalToken := common.SMTPAccount, common.SMTPFrom, common.SMTPToken
	originalSSL, originalStartTLS := common.SMTPSSLEnabled, common.SMTPStartTLSEnabled
	t.Cleanup(func() {
		common.SMTPServer, common.SMTPPort = originalServer, originalPort
		common.SMTPAccount, common.SMTPFrom, common.SMTPToken = originalAccount, originalFrom, originalToken
		common.SMTPSSLEnabled, common.SMTPStartTLSEnabled = originalSSL, originalStartTLS
	})
	model.DB.Exec("DELETE FROM invoice_application_orders")
	model.DB.Exec("DELETE FROM invoice_applications")
	model.DB.Exec("DELETE FROM invoice_titles")
	model.DB.Exec("DELETE FROM top_ups")
	title := &model.InvoiceTitle{UserID: 1101, Name: "Example Company", TaxNumber: "TEST-TAX", Address: "Example Address", Email: "invoice@example.com"}
	require.NoError(t, model.CreateInvoiceTitle(title))
	order := model.TopUp{UserId: 1101, Money: 12.5, TradeNo: "e2e-invoice-order", Status: common.TopUpStatusSuccess, InvoiceStatus: model.InvoiceStatusUnissued}
	require.NoError(t, model.DB.Create(&order).Error)
	application, err := model.CreateInvoiceApplication(1101, title.ID, []int{order.Id}, "Invoice {{invoice_id}}", "Order {{orders}}")
	require.NoError(t, err)
	host, port, messages := startInvoiceSMTP(t)
	common.SMTPServer, common.SMTPPort = host, port
	common.SMTPAccount, common.SMTPFrom, common.SMTPToken = "", "sender@example.com", ""
	common.SMTPSSLEnabled, common.SMTPStartTLSEnabled = false, false
	pdf := []byte("%PDF-1.7\nend")
	require.NoError(t, IssueInvoice(application.ID, 77, invoiceFileHeader(t, "invoice.pdf", pdf)))
	message := <-messages
	assert.Contains(t, message, "Content-Disposition: attachment")
	assert.Contains(t, message, "invoice.pdf")
	assert.Contains(t, message, "JVBERi0xLjcKZW5k")
	updated, _, err := model.GetInvoiceApplication(application.ID, 0)
	require.NoError(t, err)
	assert.Equal(t, model.InvoiceStatusIssued, updated.Status)
	assert.Equal(t, int64(len(pdf)), updated.PDFSize)
	assert.Equal(t, model.InvoiceStatusIssued, model.GetTopUpById(order.Id).InvoiceStatus)
}

func TestIssueInvoiceSMTPFailureLeavesApplicationAndOrdersIssuing(t *testing.T) {
	model.DB.Exec("DELETE FROM invoice_application_orders")
	model.DB.Exec("DELETE FROM invoice_applications")
	model.DB.Exec("DELETE FROM invoice_titles")
	model.DB.Exec("DELETE FROM top_ups")
	title := &model.InvoiceTitle{UserID: 1102, Name: "Example Company", TaxNumber: "TEST-TAX", Address: "Example Address", Email: "invoice@example.com"}
	require.NoError(t, model.CreateInvoiceTitle(title))
	order := model.TopUp{UserId: 1102, Money: 5, TradeNo: "e2e-invoice-failure", Status: common.TopUpStatusSuccess, InvoiceStatus: model.InvoiceStatusUnissued}
	require.NoError(t, model.DB.Create(&order).Error)
	application, err := model.CreateInvoiceApplication(1102, title.ID, []int{order.Id}, "subject", "body")
	require.NoError(t, err)
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	host, portText, err := net.SplitHostPort(listener.Addr().String())
	require.NoError(t, err)
	listener.Close()
	port, err := strconv.Atoi(portText)
	require.NoError(t, err)

	originalServer, originalPort := common.SMTPServer, common.SMTPPort
	originalAccount, originalFrom, originalToken := common.SMTPAccount, common.SMTPFrom, common.SMTPToken
	originalSSL, originalStartTLS := common.SMTPSSLEnabled, common.SMTPStartTLSEnabled
	t.Cleanup(func() {
		common.SMTPServer, common.SMTPPort = originalServer, originalPort
		common.SMTPAccount, common.SMTPFrom, common.SMTPToken = originalAccount, originalFrom, originalToken
		common.SMTPSSLEnabled, common.SMTPStartTLSEnabled = originalSSL, originalStartTLS
	})
	common.SMTPServer, common.SMTPPort, common.SMTPFrom = host, port, "sender@example.com"
	common.SMTPAccount, common.SMTPToken = "", ""
	common.SMTPSSLEnabled, common.SMTPStartTLSEnabled = false, false
	err = IssueInvoice(application.ID, 77, invoiceFileHeader(t, "invoice.pdf", []byte("%PDF-1.7\nfailure")))
	require.Error(t, err)
	updated, _, getErr := model.GetInvoiceApplication(application.ID, 0)
	require.NoError(t, getErr)
	assert.Equal(t, model.InvoiceStatusIssuing, updated.Status)
	assert.NotEmpty(t, updated.LastError)
	assert.Equal(t, model.InvoiceStatusIssuing, model.GetTopUpById(order.Id).InvoiceStatus)
}
