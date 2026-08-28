package model

import (
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func insertTopUpForInvoiceTest(t *testing.T, tradeNo string, userId int, status string, invoiceIssued bool) {
	t.Helper()
	require.NoError(t, DB.Create(&TopUp{
		UserId:        userId,
		Amount:        10,
		Money:         1,
		TradeNo:       tradeNo,
		PaymentMethod: PaymentMethodAlipay,
		Status:        status,
		CreateTime:    time.Now().Unix(),
		InvoiceIssued: invoiceIssued,
	}).Error)
}

func TestGetAllTopUpsFiltersByUserAndInvoiceStatus(t *testing.T) {
	truncateTables(t)

	insertTopUpForInvoiceTest(t, "invoice-filter-target", 101, common.TopUpStatusSuccess, true)
	insertTopUpForInvoiceTest(t, "invoice-filter-user", 101, common.TopUpStatusSuccess, false)
	insertTopUpForInvoiceTest(t, "invoice-filter-status", 102, common.TopUpStatusSuccess, true)

	userId := 101
	issued := true
	items, total, err := GetAllTopUps(TopUpFilter{
		Keyword:       "invoice-filter-target",
		UserId:        &userId,
		InvoiceIssued: &issued,
	}, &common.PageInfo{Page: 1, PageSize: 10})

	require.NoError(t, err)
	require.Equal(t, int64(1), total)
	require.Len(t, items, 1)
	assert.Equal(t, "invoice-filter-target", items[0].TradeNo)
}

func TestGetUserTopUpsRestrictsToUserAndInvoiceStatus(t *testing.T) {
	truncateTables(t)

	insertTopUpForInvoiceTest(t, "user-invoice-issued", 201, common.TopUpStatusSuccess, true)
	insertTopUpForInvoiceTest(t, "user-invoice-unissued", 201, common.TopUpStatusSuccess, false)
	insertTopUpForInvoiceTest(t, "other-user-invoice-unissued", 202, common.TopUpStatusSuccess, false)

	unissued := false
	items, total, err := GetUserTopUps(201, TopUpFilter{
		InvoiceIssued: &unissued,
	}, &common.PageInfo{Page: 1, PageSize: 10})

	require.NoError(t, err)
	require.Equal(t, int64(1), total)
	require.Len(t, items, 1)
	assert.Equal(t, "user-invoice-unissued", items[0].TradeNo)
}

func TestUpdateTopUpInvoiceIssuedRequiresCompletedOrder(t *testing.T) {
	truncateTables(t)

	insertTopUpForInvoiceTest(t, "invoice-success", 301, common.TopUpStatusSuccess, false)
	insertTopUpForInvoiceTest(t, "invoice-pending", 301, common.TopUpStatusPending, false)

	require.NoError(t, UpdateTopUpInvoiceIssued("invoice-success", true))
	updated := GetTopUpByTradeNo("invoice-success")
	require.NotNil(t, updated)
	assert.True(t, updated.InvoiceIssued)

	require.NoError(t, UpdateTopUpInvoiceIssued("invoice-success", false))
	updated = GetTopUpByTradeNo("invoice-success")
	require.NotNil(t, updated)
	assert.False(t, updated.InvoiceIssued)

	require.ErrorIs(t, UpdateTopUpInvoiceIssued("invoice-pending", true), ErrTopUpInvoiceIneligible)
	require.ErrorIs(t, UpdateTopUpInvoiceIssued("missing-invoice-order", true), ErrTopUpNotFound)
}

func TestUpdateTopUpInvoiceIssuedCannotOverrideIssuingOrder(t *testing.T) {
	truncateTables(t)
	require.NoError(t, DB.Create(&TopUp{
		UserId: 302, Amount: 10, Money: 1, TradeNo: "invoice-issuing",
		PaymentMethod: PaymentMethodAlipay, Status: common.TopUpStatusSuccess,
		InvoiceStatus: InvoiceStatusIssuing, CreateTime: time.Now().Unix(),
	}).Error)

	require.ErrorIs(t, UpdateTopUpInvoiceIssued("invoice-issuing", true), ErrTopUpInvoiceIneligible)
	updated := GetTopUpByTradeNo("invoice-issuing")
	require.NotNil(t, updated)
	assert.Equal(t, InvoiceStatusIssuing, updated.InvoiceStatus)
	assert.False(t, updated.InvoiceIssued)
}
