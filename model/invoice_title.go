package model

import (
	"errors"
	"net/mail"
	"strings"
	"time"

	"gorm.io/gorm"
)

var (
	ErrInvoiceTitleNotFound = errors.New("invoice title not found")
	ErrInvoiceTitleInvalid  = errors.New("invoice title fields are required")
)

type InvoiceTitle struct {
	ID        int    `json:"id" gorm:"primaryKey"`
	UserID    int    `json:"user_id" gorm:"index;not null"`
	Name      string `json:"name" gorm:"type:varchar(255);not null"`
	TaxNumber string `json:"tax_number" gorm:"type:varchar(64);not null"`
	Address   string `json:"address" gorm:"type:varchar(500);not null"`
	Email     string `json:"email" gorm:"type:varchar(255);not null"`
	CreatedAt int64  `json:"created_at" gorm:"bigint"`
	UpdatedAt int64  `json:"updated_at" gorm:"bigint"`
}

func (title *InvoiceTitle) Normalize() error {
	if title == nil {
		return ErrInvoiceTitleInvalid
	}
	title.Name = strings.TrimSpace(title.Name)
	title.TaxNumber = strings.TrimSpace(title.TaxNumber)
	title.Address = strings.TrimSpace(title.Address)
	title.Email = NormalizeEmail(title.Email)
	parsedEmail, err := mail.ParseAddress(title.Email)
	if title.UserID <= 0 || title.Name == "" || len(title.Name) > 255 || title.TaxNumber == "" || len(title.TaxNumber) > 64 || title.Address == "" || len(title.Address) > 500 || len(title.Email) > 255 || err != nil || parsedEmail.Address != title.Email {
		return ErrInvoiceTitleInvalid
	}
	return nil
}

func ListInvoiceTitles(userID int) ([]InvoiceTitle, error) {
	var titles []InvoiceTitle
	err := DB.Where("user_id = ?", userID).Order("id desc").Find(&titles).Error
	return titles, err
}

func GetInvoiceTitleForUser(tx *gorm.DB, userID, titleID int) (*InvoiceTitle, error) {
	if tx == nil {
		tx = DB
	}
	var title InvoiceTitle
	if err := tx.Where("id = ? AND user_id = ?", titleID, userID).First(&title).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrInvoiceTitleNotFound
		}
		return nil, err
	}
	return &title, nil
}

func CreateInvoiceTitle(title *InvoiceTitle) error {
	if err := title.Normalize(); err != nil {
		return err
	}
	now := time.Now().Unix()
	title.CreatedAt, title.UpdatedAt = now, now
	return DB.Create(title).Error
}

func UpdateInvoiceTitle(title *InvoiceTitle) error {
	if err := title.Normalize(); err != nil {
		return err
	}
	title.UpdatedAt = time.Now().Unix()
	return DB.Model(&InvoiceTitle{}).Where("id = ? AND user_id = ?", title.ID, title.UserID).
		Updates(map[string]interface{}{"name": title.Name, "tax_number": title.TaxNumber, "address": title.Address, "email": title.Email, "updated_at": title.UpdatedAt}).Error
}

func DeleteInvoiceTitle(userID, titleID int) error {
	result := DB.Where("id = ? AND user_id = ?", titleID, userID).Delete(&InvoiceTitle{})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return ErrInvoiceTitleNotFound
	}
	return nil
}
