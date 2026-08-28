package model

import (
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

const (
	InvoiceStatusUnissued = "unissued"
	InvoiceStatusIssuing  = "issuing"
	InvoiceStatusIssued   = "issued"
	invoiceSendLeaseTTL   = 10 * time.Minute
)

var (
	ErrInvoiceApplicationInvalid  = errors.New("invalid invoice application")
	ErrInvoiceOrderIneligible     = errors.New("invoice order is not eligible")
	ErrInvoiceApplicationNotFound = errors.New("invoice application not found")
	ErrInvoiceApplicationIssued   = errors.New("invoice application already issued")
	ErrInvoiceApplicationSending  = errors.New("invoice application is already being sent")
	ErrInvoiceApplicationNotReady = errors.New("invoice application is not ready to send")
	ErrInvoiceSendLeaseLost       = errors.New("invoice send lease was lost")
)

type InvoiceApplication struct {
	ID             int     `json:"id" gorm:"primaryKey"`
	UserID         int     `json:"user_id" gorm:"index;not null"`
	TitleID        int     `json:"title_id" gorm:"index;not null"`
	Status         string  `json:"status" gorm:"type:varchar(20);index;not null"`
	TotalMoney     float64 `json:"total_money"`
	RecipientEmail string  `json:"recipient_email" gorm:"type:varchar(255);not null"`
	TitleName      string  `json:"title_name" gorm:"type:varchar(255);not null"`
	TitleTaxNumber string  `json:"title_tax_number" gorm:"type:varchar(64);not null"`
	TitleAddress   string  `json:"title_address" gorm:"type:varchar(500);not null"`
	EmailSubject   string  `json:"email_subject" gorm:"type:varchar(500)"`
	EmailBody      string  `json:"email_body" gorm:"type:text"`
	PDFFileName    string  `json:"pdf_file_name" gorm:"type:varchar(255)"`
	PDFSHA256      string  `json:"pdf_sha256" gorm:"column:pdf_sha256;type:char(64)"`
	PDFSize        int64   `json:"pdf_size"`
	LastError      string  `json:"last_error" gorm:"type:text"`
	CreatedAt      int64   `json:"created_at" gorm:"bigint;index"`
	UpdatedAt      int64   `json:"updated_at" gorm:"bigint"`
	IssuedAt       int64   `json:"issued_at" gorm:"bigint"`
	IssuedBy       int     `json:"issued_by"`
	SendingAt      int64   `json:"-" gorm:"bigint;index"`
	SendingToken   string  `json:"-" gorm:"type:varchar(64);index"`
}

type InvoiceApplicationOrder struct {
	ID            int     `json:"id" gorm:"primaryKey"`
	ApplicationID int     `json:"application_id" gorm:"uniqueIndex:idx_invoice_application_topup;not null"`
	TopUpID       int     `json:"topup_id" gorm:"uniqueIndex:idx_invoice_application_topup;index;not null"`
	TradeNo       string  `json:"trade_no" gorm:"type:varchar(255);index"`
	Money         float64 `json:"money"`
	Amount        int64   `json:"amount"`
}

type InvoiceApplicationFilter struct {
	Status  string
	UserID  *int
	Keyword string
}

func normalizeInvoiceStatus(status string) string {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case InvoiceStatusIssuing:
		return InvoiceStatusIssuing
	case InvoiceStatusIssued:
		return InvoiceStatusIssued
	case InvoiceStatusUnissued:
		return InvoiceStatusUnissued
	default:
		return ""
	}
}

func GetInvoiceApplication(id, userID int) (*InvoiceApplication, []InvoiceApplicationOrder, error) {
	var application InvoiceApplication
	query := DB.Where("id = ?", id)
	if userID > 0 {
		query = query.Where("user_id = ?", userID)
	}
	if err := query.First(&application).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil, ErrInvoiceApplicationNotFound
		}
		return nil, nil, err
	}
	var orders []InvoiceApplicationOrder
	if err := DB.Where("application_id = ?", application.ID).Order("id asc").Find(&orders).Error; err != nil {
		return nil, nil, err
	}
	return &application, orders, nil
}

func ListInvoiceApplications(filter InvoiceApplicationFilter, pageInfo *common.PageInfo) ([]InvoiceApplication, int64, error) {
	query := DB.Model(&InvoiceApplication{})
	if filter.Status != "" {
		status := normalizeInvoiceStatus(filter.Status)
		if status == "" {
			return nil, 0, fmt.Errorf("invalid invoice status")
		}
		query = query.Where("status = ?", status)
	}
	if filter.UserID != nil {
		query = query.Where("user_id = ?", *filter.UserID)
	}
	if strings.TrimSpace(filter.Keyword) != "" {
		keyword := strings.TrimSpace(filter.Keyword)
		pattern, err := sanitizeLikePattern("%" + keyword + "%")
		if err != nil {
			return nil, 0, err
		}
		keywordQuery := DB.Model(&InvoiceApplicationOrder{}).Select("application_id").Where("trade_no LIKE ? ESCAPE '!'", pattern)
		keywordCondition := DB.Where("recipient_email LIKE ? ESCAPE '!' OR id IN (?)", pattern, keywordQuery)
		if applicationID, err := strconv.Atoi(keyword); err == nil && applicationID > 0 {
			keywordCondition = keywordCondition.Or("id = ?", applicationID)
		}
		query = query.Where(keywordCondition)
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var items []InvoiceApplication
	err := query.Order("id desc").Limit(pageInfo.GetPageSize()).Offset(pageInfo.GetStartIdx()).Find(&items).Error
	return items, total, err
}

func CreateInvoiceApplication(userID, titleID int, topupIDs []int, subject, body string) (*InvoiceApplication, error) {
	if userID <= 0 || titleID <= 0 || len(topupIDs) == 0 {
		return nil, ErrInvoiceApplicationInvalid
	}
	ids := append([]int(nil), topupIDs...)
	sort.Ints(ids)
	for i := 1; i < len(ids); i++ {
		if ids[i] == ids[i-1] {
			return nil, ErrInvoiceApplicationInvalid
		}
	}
	var application InvoiceApplication
	err := DB.Transaction(func(tx *gorm.DB) error {
		title, err := GetInvoiceTitleForUser(tx, userID, titleID)
		if err != nil {
			return err
		}
		var orders []TopUp
		if err := lockForUpdate(tx).Where("id IN ?", ids).Find(&orders).Error; err != nil {
			return err
		}
		if len(orders) != len(ids) {
			return ErrInvoiceOrderIneligible
		}
		byID := make(map[int]TopUp, len(orders))
		for _, order := range orders {
			byID[order.Id] = order
		}
		total := decimal.Zero
		links := make([]InvoiceApplicationOrder, 0, len(ids))
		for _, id := range ids {
			order, ok := byID[id]
			if !ok || order.UserId != userID || order.Status != common.TopUpStatusSuccess || invoiceStatusOf(&order) != InvoiceStatusUnissued {
				return ErrInvoiceOrderIneligible
			}
			total = total.Add(decimal.NewFromFloat(order.Money))
			links = append(links, InvoiceApplicationOrder{TopUpID: order.Id, TradeNo: order.TradeNo, Money: order.Money, Amount: order.Amount})
		}
		now := common.GetTimestamp()
		application = InvoiceApplication{UserID: userID, TitleID: titleID, Status: InvoiceStatusIssuing, TotalMoney: total.InexactFloat64(), RecipientEmail: title.Email, TitleName: title.Name, TitleTaxNumber: title.TaxNumber, TitleAddress: title.Address, EmailSubject: subject, EmailBody: body, CreatedAt: now, UpdatedAt: now}
		if err := tx.Create(&application).Error; err != nil {
			return err
		}
		for i := range links {
			links[i].ApplicationID = application.ID
		}
		if err := tx.Create(&links).Error; err != nil {
			return err
		}
		return tx.Model(&TopUp{}).Where("id IN ?", ids).Updates(map[string]interface{}{"invoice_status": InvoiceStatusIssuing, "invoice_issued": false}).Error
	})
	if err != nil {
		return nil, err
	}
	return &application, nil
}

func invoiceStatusOf(order *TopUp) string {
	if order.InvoiceStatus == InvoiceStatusIssuing || order.InvoiceStatus == InvoiceStatusIssued {
		return order.InvoiceStatus
	}
	if order.InvoiceIssued {
		return InvoiceStatusIssued
	}
	return InvoiceStatusUnissued
}

func SetInvoiceApplicationFailure(id int, sendingToken, message string) error {
	return DB.Transaction(func(tx *gorm.DB) error {
		var application InvoiceApplication
		if err := lockForUpdate(tx).Where("id = ?", id).First(&application).Error; err != nil {
			return ErrInvoiceApplicationNotFound
		}
		if application.Status == InvoiceStatusIssued {
			return nil
		}
		if application.SendingToken != sendingToken || sendingToken == "" {
			return ErrInvoiceSendLeaseLost
		}
		return tx.Model(&application).Updates(map[string]interface{}{"status": InvoiceStatusIssuing, "last_error": message, "sending_at": 0, "sending_token": "", "updated_at": common.GetTimestamp()}).Error
	})
}

func BeginInvoiceApplicationIssuance(id int) (*InvoiceApplication, []InvoiceApplicationOrder, error) {
	var application InvoiceApplication
	var links []InvoiceApplicationOrder
	err := DB.Transaction(func(tx *gorm.DB) error {
		if err := lockForUpdate(tx).Where("id = ?", id).First(&application).Error; err != nil {
			return ErrInvoiceApplicationNotFound
		}
		if application.Status == InvoiceStatusIssued {
			return ErrInvoiceApplicationIssued
		}
		if application.Status != InvoiceStatusIssuing {
			return ErrInvoiceApplicationNotReady
		}
		now := common.GetTimestamp()
		if application.SendingToken != "" && application.SendingAt > now-int64(invoiceSendLeaseTTL/time.Second) {
			return ErrInvoiceApplicationSending
		}
		if err := tx.Where("application_id = ?", id).Order("id asc").Find(&links).Error; err != nil {
			return err
		}
		if len(links) == 0 {
			return ErrInvoiceApplicationInvalid
		}
		ids := make([]int, 0, len(links))
		for _, link := range links {
			ids = append(ids, link.TopUpID)
		}
		var orders []TopUp
		if err := lockForUpdate(tx).Where("id IN ?", ids).Find(&orders).Error; err != nil {
			return err
		}
		if len(orders) != len(ids) {
			return ErrInvoiceOrderIneligible
		}
		for _, order := range orders {
			if invoiceStatusOf(&order) != InvoiceStatusIssuing {
				return ErrInvoiceOrderIneligible
			}
		}
		sendingToken := common.GetRandomString(32)
		if err := tx.Model(&application).Updates(map[string]interface{}{"sending_at": now, "sending_token": sendingToken, "last_error": "", "updated_at": now}).Error; err != nil {
			return err
		}
		application.SendingAt = now
		application.SendingToken = sendingToken
		return nil
	})
	return &application, links, err
}

func SetInvoiceApplicationIssued(id, adminID int, sendingToken, fileName, sha256 string, fileSize int64) error {
	return DB.Transaction(func(tx *gorm.DB) error {
		var application InvoiceApplication
		if err := lockForUpdate(tx).Where("id = ?", id).First(&application).Error; err != nil {
			return ErrInvoiceApplicationNotFound
		}
		if application.Status == InvoiceStatusIssued {
			return ErrInvoiceApplicationIssued
		}
		if application.Status != InvoiceStatusIssuing || application.SendingToken != sendingToken || sendingToken == "" {
			return ErrInvoiceSendLeaseLost
		}
		var links []InvoiceApplicationOrder
		if err := tx.Where("application_id = ?", id).Find(&links).Error; err != nil {
			return err
		}
		ids := make([]int, 0, len(links))
		for _, link := range links {
			ids = append(ids, link.TopUpID)
		}
		if len(ids) == 0 {
			return ErrInvoiceApplicationInvalid
		}
		var orders []TopUp
		if err := lockForUpdate(tx).Where("id IN ?", ids).Find(&orders).Error; err != nil {
			return err
		}
		if len(orders) != len(ids) {
			return ErrInvoiceOrderIneligible
		}
		for _, order := range orders {
			if invoiceStatusOf(&order) != InvoiceStatusIssuing {
				return ErrInvoiceOrderIneligible
			}
		}
		now := common.GetTimestamp()
		if err := tx.Model(&application).Updates(map[string]interface{}{"status": InvoiceStatusIssued, "pdf_file_name": fileName, "pdf_sha256": sha256, "pdf_size": fileSize, "last_error": "", "issued_at": now, "issued_by": adminID, "sending_at": 0, "sending_token": "", "updated_at": now}).Error; err != nil {
			return err
		}
		return tx.Model(&TopUp{}).Where("id IN ?", ids).Updates(map[string]interface{}{"invoice_status": InvoiceStatusIssued, "invoice_issued": true}).Error
	})
}

func ListInvoiceOrders(userID int, status string, pageInfo *common.PageInfo) ([]TopUp, int64, error) {
	query := DB.Model(&TopUp{}).Where("user_id = ? AND status = ?", userID, common.TopUpStatusSuccess)
	if status != "" && status != "all" {
		normalized := normalizeInvoiceStatus(status)
		if normalized == "" {
			return nil, 0, fmt.Errorf("invalid invoice status")
		}
		switch normalized {
		case InvoiceStatusUnissued:
			query = query.Where("invoice_status = ? OR ((invoice_status IS NULL OR invoice_status = ?) AND invoice_issued = ?)", InvoiceStatusUnissued, "", false)
		case InvoiceStatusIssued:
			query = query.Where("invoice_status = ? OR ((invoice_status IS NULL OR invoice_status = ?) AND invoice_issued = ?)", InvoiceStatusIssued, "", true)
		default:
			query = query.Where("invoice_status = ?", normalized)
		}
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var orders []TopUp
	err := query.Order("id desc").Limit(pageInfo.GetPageSize()).Offset(pageInfo.GetStartIdx()).Find(&orders).Error
	for i := range orders {
		orders[i].InvoiceStatus = invoiceStatusOf(&orders[i])
	}
	return orders, total, err
}
