package service

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"html/template"
	"io"
	"mime/multipart"
	"path/filepath"
	"strings"
	"unicode"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
)

const (
	InvoiceDefaultSubject = "发票开具通知（{{invoice_id}}）"
	InvoiceDefaultBody    = "尊敬的 {{title_name}}：<br><br>您好！您申请开具的发票现已完成开具，发票金额为 {{total_money}}。发票文件已随本邮件附上，请查收。<br><br>关联订单：<br>{{orders}}<br><br>本邮件由系统自动发送，请勿直接回复。如有疑问，请联系平台客服。<br>{{system_name}}"
	InvoiceMaxPDFSize     = 10 << 20
)

var ErrInvoicePDFInvalid = errors.New("a PDF file no larger than 10 MiB is required")

type InvoiceTemplate struct {
	Subject string `json:"subject"`
	Body    string `json:"body"`
}

func CurrentInvoiceTemplate() InvoiceTemplate {
	result := InvoiceTemplate{Subject: InvoiceDefaultSubject, Body: InvoiceDefaultBody}
	common.OptionMapRWMutex.RLock()
	if value := strings.TrimSpace(common.OptionMap["invoice_email_subject"]); value != "" {
		result.Subject = value
	}
	if value := strings.TrimSpace(common.OptionMap["invoice_email_body"]); value != "" {
		result.Body = value
	}
	common.OptionMapRWMutex.RUnlock()
	return result
}

func SaveInvoiceTemplate(value InvoiceTemplate) error {
	value.Subject = strings.TrimSpace(value.Subject)
	value.Body = strings.TrimSpace(value.Body)
	if value.Subject == "" || value.Body == "" || len(value.Subject) > 500 || len(value.Body) > 20000 || strings.ContainsAny(value.Subject, "\r\n") {
		return errors.New("invoice email subject and body are required")
	}
	return model.UpdateOptionsBulk(map[string]string{"invoice_email_subject": value.Subject, "invoice_email_body": value.Body})
}

func ValidateInvoicePDF(header *multipart.FileHeader) ([]byte, string, int64, error) {
	if header == nil || header.Size <= 0 || header.Size > InvoiceMaxPDFSize || !strings.EqualFold(filepath.Ext(header.Filename), ".pdf") {
		return nil, "", 0, ErrInvoicePDFInvalid
	}
	file, err := header.Open()
	if err != nil {
		return nil, "", 0, err
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, InvoiceMaxPDFSize+1))
	if err != nil {
		return nil, "", 0, err
	}
	if len(data) == 0 || len(data) > InvoiceMaxPDFSize || !strings.HasPrefix(string(data), "%PDF-") {
		return nil, "", 0, ErrInvoicePDFInvalid
	}
	name := sanitizeInvoiceFilename(filepath.Base(header.Filename))
	if name == "" {
		name = "invoice.pdf"
	}
	return data, name, int64(len(data)), nil
}

func sanitizeInvoiceFilename(name string) string {
	name = strings.ReplaceAll(name, "\\", "/")
	name = filepath.Base(name)
	name = strings.Map(func(r rune) rune {
		if r == '"' || r == '\\' || unicode.IsControl(r) {
			return -1
		}
		return r
	}, name)
	return strings.TrimSpace(name)
}

func RenderInvoiceTemplate(raw InvoiceTemplate, application *model.InvoiceApplication, orders []model.InvoiceApplicationOrder) (string, string, error) {
	if application == nil {
		return "", "", model.ErrInvoiceApplicationNotFound
	}
	if strings.TrimSpace(raw.Subject) == "" {
		raw.Subject = InvoiceDefaultSubject
	}
	if strings.TrimSpace(raw.Body) == "" {
		raw.Body = InvoiceDefaultBody
	}
	orderLines := make([]string, 0, len(orders))
	for _, order := range orders {
		orderLines = append(orderLines, template.HTMLEscapeString(fmt.Sprintf("%s (%.2f)", order.TradeNo, order.Money)))
	}
	data := map[string]string{
		"invoice_id":      fmt.Sprint(application.ID),
		"recipient_email": template.HTMLEscapeString(application.RecipientEmail),
		"title_name":      template.HTMLEscapeString(application.TitleName),
		"total_money":     fmt.Sprintf("%.2f", application.TotalMoney),
		"orders":          strings.Join(orderLines, "<br>"),
		"system_name":     template.HTMLEscapeString(common.SystemName),
	}
	replace := func(input string) string {
		for key, value := range data {
			input = strings.ReplaceAll(input, "{{"+key+"}}", value)
		}
		return input
	}
	return replace(raw.Subject), replace(raw.Body), nil
}

func IssueInvoice(applicationID, adminID int, header *multipart.FileHeader) error {
	data, fileName, fileSize, err := ValidateInvoicePDF(header)
	if err != nil {
		return err
	}
	application, orders, err := model.BeginInvoiceApplicationIssuance(applicationID)
	if err != nil {
		return err
	}
	renderedSubject, renderedBody, err := RenderInvoiceTemplate(InvoiceTemplate{Subject: application.EmailSubject, Body: application.EmailBody}, application, orders)
	if err != nil {
		_ = model.SetInvoiceApplicationFailure(applicationID, application.SendingToken, err.Error())
		return err
	}
	if err := common.SendEmailWithAttachment(renderedSubject, application.RecipientEmail, renderedBody, fileName, "application/pdf", data); err != nil {
		_ = model.SetInvoiceApplicationFailure(applicationID, application.SendingToken, err.Error())
		return err
	}
	checksum := fmt.Sprintf("%x", sha256.Sum256(data))
	return model.SetInvoiceApplicationIssued(applicationID, adminID, application.SendingToken, fileName, checksum, fileSize)
}
