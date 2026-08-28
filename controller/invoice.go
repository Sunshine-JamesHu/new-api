package controller

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
)

type invoiceTitleRequest struct {
	Name      string `json:"name"`
	TaxNumber string `json:"tax_number"`
	Address   string `json:"address"`
	Email     string `json:"email"`
}

type invoiceApplicationRequest struct {
	TitleID  int   `json:"title_id"`
	TopUpIDs []int `json:"topup_ids"`
}

type invoiceTemplateRequest struct {
	Subject string `json:"subject"`
	Body    string `json:"body"`
}

func invoiceError(c *gin.Context, err error) {
	status := http.StatusBadRequest
	if errors.Is(err, model.ErrInvoiceTitleNotFound) || errors.Is(err, model.ErrInvoiceApplicationNotFound) {
		status = http.StatusNotFound
	}
	if strings.Contains(err.Error(), "already") || strings.Contains(err.Error(), "being sent") {
		status = http.StatusConflict
	}
	if errors.Is(err, model.ErrInvoiceSendLeaseLost) {
		status = http.StatusConflict
	}
	c.JSON(status, gin.H{"success": false, "message": err.Error()})
}

func GetInvoiceTitles(c *gin.Context) {
	titles, err := model.ListInvoiceTitles(c.GetInt("id"))
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, gin.H{"items": titles})
}

func CreateInvoiceTitle(c *gin.Context) {
	var request invoiceTitleRequest
	if err := common.DecodeJson(c.Request.Body, &request); err != nil {
		invoiceError(c, model.ErrInvoiceTitleInvalid)
		return
	}
	title := &model.InvoiceTitle{UserID: c.GetInt("id"), Name: request.Name, TaxNumber: request.TaxNumber, Address: request.Address, Email: request.Email}
	if err := model.CreateInvoiceTitle(title); err != nil {
		invoiceError(c, err)
		return
	}
	common.ApiSuccess(c, title)
}

func UpdateInvoiceTitle(c *gin.Context) {
	titleID, err := strconv.Atoi(c.Param("id"))
	if err != nil || titleID <= 0 {
		invoiceError(c, model.ErrInvoiceTitleNotFound)
		return
	}
	var request invoiceTitleRequest
	if err := common.DecodeJson(c.Request.Body, &request); err != nil {
		invoiceError(c, model.ErrInvoiceTitleInvalid)
		return
	}
	title := &model.InvoiceTitle{ID: titleID, UserID: c.GetInt("id"), Name: request.Name, TaxNumber: request.TaxNumber, Address: request.Address, Email: request.Email}
	if _, err := model.GetInvoiceTitleForUser(nil, title.UserID, title.ID); err != nil {
		invoiceError(c, err)
		return
	}
	if err := model.UpdateInvoiceTitle(title); err != nil {
		invoiceError(c, err)
		return
	}
	common.ApiSuccess(c, title)
}

func DeleteInvoiceTitle(c *gin.Context) {
	titleID, err := strconv.Atoi(c.Param("id"))
	if err != nil || titleID <= 0 {
		invoiceError(c, model.ErrInvoiceTitleNotFound)
		return
	}
	if err := model.DeleteInvoiceTitle(c.GetInt("id"), titleID); err != nil {
		invoiceError(c, err)
		return
	}
	common.ApiSuccess(c, nil)
}

func GetInvoiceOrders(c *gin.Context) {
	pageInfo := common.GetPageQuery(c)
	status := c.DefaultQuery("status", model.InvoiceStatusUnissued)
	orders, total, err := model.ListInvoiceOrders(c.GetInt("id"), status, pageInfo)
	if err != nil {
		invoiceError(c, err)
		return
	}
	pageInfo.SetTotal(int(total))
	pageInfo.SetItems(orders)
	common.ApiSuccess(c, pageInfo)
}

func CreateInvoiceApplication(c *gin.Context) {
	var request invoiceApplicationRequest
	if err := common.DecodeJson(c.Request.Body, &request); err != nil {
		invoiceError(c, model.ErrInvoiceApplicationInvalid)
		return
	}
	templateConfig := service.CurrentInvoiceTemplate()
	application, err := model.CreateInvoiceApplication(c.GetInt("id"), request.TitleID, request.TopUpIDs, templateConfig.Subject, templateConfig.Body)
	if err != nil {
		invoiceError(c, err)
		return
	}
	common.ApiSuccess(c, application)
}

func GetInvoiceApplications(c *gin.Context) {
	pageInfo := common.GetPageQuery(c)
	status := c.Query("status")
	if status == "all" {
		status = ""
	}
	userID := c.GetInt("id")
	items, total, err := model.ListInvoiceApplications(model.InvoiceApplicationFilter{Status: status, UserID: &userID}, pageInfo)
	if err != nil {
		invoiceError(c, err)
		return
	}
	// User endpoint must never expose another user's application.
	filtered := make([]model.InvoiceApplication, 0, len(items))
	for _, item := range items {
		if item.UserID == userID {
			filtered = append(filtered, item)
		}
	}
	pageInfo.SetTotal(int(total))
	pageInfo.SetItems(filtered)
	common.ApiSuccess(c, pageInfo)
}

func GetAdminInvoiceApplications(c *gin.Context) {
	pageInfo := common.GetPageQuery(c)
	filter := model.InvoiceApplicationFilter{Status: c.Query("status"), Keyword: c.Query("keyword")}
	if filter.Status == "all" {
		filter.Status = ""
	}
	if rawUserID := strings.TrimSpace(c.Query("user_id")); rawUserID != "" {
		userID, err := strconv.Atoi(rawUserID)
		if err != nil || userID <= 0 {
			invoiceError(c, errors.New("user ID must be a positive integer"))
			return
		}
		filter.UserID = &userID
	}
	items, total, err := model.ListInvoiceApplications(filter, pageInfo)
	if err != nil {
		invoiceError(c, err)
		return
	}
	pageInfo.SetTotal(int(total))
	pageInfo.SetItems(items)
	common.ApiSuccess(c, pageInfo)
}

func GetInvoiceApplicationDetail(c *gin.Context) {
	applicationID, err := strconv.Atoi(c.Param("id"))
	if err != nil || applicationID <= 0 {
		invoiceError(c, model.ErrInvoiceApplicationNotFound)
		return
	}
	userID := 0
	if !c.GetBool("is_admin") {
		userID = c.GetInt("id")
	}
	application, orders, err := model.GetInvoiceApplication(applicationID, userID)
	if err != nil {
		invoiceError(c, err)
		return
	}
	common.ApiSuccess(c, gin.H{"application": application, "orders": orders})
}

func GetInvoiceTemplate(c *gin.Context) {
	common.ApiSuccess(c, service.CurrentInvoiceTemplate())
}

func UpdateInvoiceTemplate(c *gin.Context) {
	var request invoiceTemplateRequest
	if err := common.DecodeJson(c.Request.Body, &request); err != nil {
		invoiceError(c, errors.New("invalid invoice template"))
		return
	}
	if err := service.SaveInvoiceTemplate(service.InvoiceTemplate{Subject: request.Subject, Body: request.Body}); err != nil {
		invoiceError(c, err)
		return
	}
	common.ApiSuccess(c, service.CurrentInvoiceTemplate())
}

func IssueInvoice(c *gin.Context) {
	applicationID, err := strconv.Atoi(c.Param("id"))
	if err != nil || applicationID <= 0 {
		invoiceError(c, model.ErrInvoiceApplicationNotFound)
		return
	}
	header, err := c.FormFile("file")
	if err != nil {
		invoiceError(c, service.ErrInvoicePDFInvalid)
		return
	}
	if err := service.IssueInvoice(applicationID, c.GetInt("id"), header); err != nil {
		invoiceError(c, err)
		return
	}
	common.ApiSuccess(c, nil)
}
