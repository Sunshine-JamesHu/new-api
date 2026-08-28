package model

import (
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCreateInvoiceApplicationClaimsEligibleOrdersAtomically(t *testing.T) {
	truncateTables(t)
	title := &InvoiceTitle{UserID: 901, Name: "Example Company", TaxNumber: "TEST-TAX-ID", Address: "Example registered address", Email: "Invoice@Example.com"}
	require.NoError(t, CreateInvoiceTitle(title))
	orders := []TopUp{
		{UserId: 901, Amount: 10, Money: 10.25, TradeNo: "invoice-batch-1", PaymentMethod: PaymentMethodAlipay, Status: common.TopUpStatusSuccess, InvoiceStatus: InvoiceStatusUnissued, CreateTime: time.Now().Unix()},
		{UserId: 901, Amount: 20, Money: 20.50, TradeNo: "invoice-batch-2", PaymentMethod: PaymentMethodAlipay, Status: common.TopUpStatusSuccess, InvoiceStatus: InvoiceStatusUnissued, CreateTime: time.Now().Unix()},
	}
	require.NoError(t, DB.Create(&orders).Error)

	application, err := CreateInvoiceApplication(901, title.ID, []int{orders[1].Id, orders[0].Id}, "subject", "body")
	require.NoError(t, err)
	assert.Equal(t, InvoiceStatusIssuing, application.Status)
	assert.InDelta(t, 30.75, application.TotalMoney, 0.001)
	assert.Equal(t, "invoice@example.com", application.RecipientEmail)

	var claimed []TopUp
	require.NoError(t, DB.Where("id IN ?", []int{orders[0].Id, orders[1].Id}).Find(&claimed).Error)
	for _, order := range claimed {
		assert.Equal(t, InvoiceStatusIssuing, order.InvoiceStatus)
		assert.False(t, order.InvoiceIssued)
	}
}

func TestCreateInvoiceApplicationRollsBackWhenAnyOrderIsIneligible(t *testing.T) {
	truncateTables(t)
	title := &InvoiceTitle{UserID: 902, Name: "Company", TaxNumber: "TAX", Address: "Address", Email: "invoice@example.com"}
	require.NoError(t, CreateInvoiceTitle(title))
	orders := []TopUp{
		{UserId: 902, Money: 1, TradeNo: "eligible", Status: common.TopUpStatusSuccess, InvoiceStatus: InvoiceStatusUnissued, CreateTime: time.Now().Unix()},
		{UserId: 903, Money: 2, TradeNo: "foreign", Status: common.TopUpStatusSuccess, InvoiceStatus: InvoiceStatusUnissued, CreateTime: time.Now().Unix()},
	}
	require.NoError(t, DB.Create(&orders).Error)

	_, err := CreateInvoiceApplication(902, title.ID, []int{orders[0].Id, orders[1].Id}, "subject", "body")
	require.ErrorIs(t, err, ErrInvoiceOrderIneligible)
	var count int64
	require.NoError(t, DB.Model(&InvoiceApplication{}).Count(&count).Error)
	assert.Zero(t, count)
	var eligible TopUp
	require.NoError(t, DB.First(&eligible, orders[0].Id).Error)
	assert.Equal(t, InvoiceStatusUnissued, eligible.InvoiceStatus)
}

func TestInvoiceFailureReleasesOrdersAndIssuedSynchronizesLegacyFlag(t *testing.T) {
	truncateTables(t)
	title := &InvoiceTitle{UserID: 904, Name: "Company", TaxNumber: "TAX", Address: "Address", Email: "invoice@example.com"}
	require.NoError(t, CreateInvoiceTitle(title))
	order := TopUp{UserId: 904, Money: 9.99, TradeNo: "retry-order", Status: common.TopUpStatusSuccess, InvoiceStatus: InvoiceStatusUnissued, CreateTime: time.Now().Unix()}
	require.NoError(t, DB.Create(&order).Error)
	application, err := CreateInvoiceApplication(904, title.ID, []int{order.Id}, "subject", "body")
	require.NoError(t, err)

	claimed, _, err := BeginInvoiceApplicationIssuance(application.ID)
	require.NoError(t, err)
	assert.NotZero(t, claimed.SendingAt)
	require.Error(t, func() error { _, _, err := BeginInvoiceApplicationIssuance(application.ID); return err }())
	require.NoError(t, SetInvoiceApplicationFailure(application.ID, claimed.SendingToken, "smtp failed"))
	retry, _, err := BeginInvoiceApplicationIssuance(application.ID)
	require.NoError(t, err)
	assert.Equal(t, InvoiceStatusIssuing, retry.Status)
	require.NoError(t, SetInvoiceApplicationIssued(application.ID, 1, retry.SendingToken, "invoice.pdf", "sha", 100))

	updated := GetTopUpById(order.Id)
	require.NotNil(t, updated)
	assert.Equal(t, InvoiceStatusIssued, updated.InvoiceStatus)
	assert.True(t, updated.InvoiceIssued)
	assert.Equal(t, InvoiceStatusIssued, updated.InvoiceStatus)
}

func TestLegacyInvoiceIssuedBooleanKeepsItsOriginalValues(t *testing.T) {
	truncateTables(t)
	falseOrder := TopUp{UserId: 905, Money: 1, TradeNo: "legacy-unissued", Status: common.TopUpStatusSuccess, InvoiceIssued: false, InvoiceStatus: InvoiceStatusUnissued, CreateTime: time.Now().Unix()}
	trueOrder := TopUp{UserId: 905, Money: 2, TradeNo: "legacy-issued", Status: common.TopUpStatusSuccess, InvoiceIssued: true, InvoiceStatus: InvoiceStatusIssued, CreateTime: time.Now().Unix()}
	require.NoError(t, DB.Create(&falseOrder).Error)
	require.NoError(t, DB.Create(&trueOrder).Error)

	var stored TopUp
	require.NoError(t, DB.Unscoped().Where("id = ?", falseOrder.Id).First(&stored).Error)
	assert.False(t, stored.InvoiceIssued)
	assert.Equal(t, InvoiceStatusUnissued, stored.InvoiceStatus)
	stored = TopUp{}
	require.NoError(t, DB.Unscoped().Where("id = ?", trueOrder.Id).First(&stored).Error)
	assert.True(t, stored.InvoiceIssued)
	assert.Equal(t, InvoiceStatusIssued, stored.InvoiceStatus)
}

func TestListInvoiceOrdersSupportsAllInvoiceStatusesAndLegacyFlags(t *testing.T) {
	truncateTables(t)
	orders := []TopUp{
		{UserId: 906, Money: 1, TradeNo: "status-unissued", Status: common.TopUpStatusSuccess, InvoiceStatus: InvoiceStatusUnissued, CreateTime: time.Now().Unix()},
		{UserId: 906, Money: 2, TradeNo: "status-issuing", Status: common.TopUpStatusSuccess, InvoiceStatus: InvoiceStatusIssuing, CreateTime: time.Now().Unix()},
		{UserId: 906, Money: 3, TradeNo: "status-issued", Status: common.TopUpStatusSuccess, InvoiceStatus: InvoiceStatusIssued, InvoiceIssued: true, CreateTime: time.Now().Unix()},
		{UserId: 906, Money: 4, TradeNo: "legacy-unissued", Status: common.TopUpStatusSuccess, InvoiceStatus: "", InvoiceIssued: false, CreateTime: time.Now().Unix()},
		{UserId: 906, Money: 5, TradeNo: "legacy-issued", Status: common.TopUpStatusSuccess, InvoiceStatus: "", InvoiceIssued: true, CreateTime: time.Now().Unix()},
	}
	require.NoError(t, DB.Create(&orders).Error)
	for _, check := range []struct {
		status string
		want   []string
	}{
		{InvoiceStatusUnissued, []string{"legacy-unissued", "status-unissued"}},
		{InvoiceStatusIssuing, []string{"status-issuing"}},
		{InvoiceStatusIssued, []string{"legacy-issued", "status-issued"}},
	} {
		items, _, err := ListInvoiceOrders(906, check.status, &common.PageInfo{Page: 1, PageSize: 20})
		require.NoError(t, err)
		got := make([]string, 0, len(items))
		for _, item := range items {
			got = append(got, item.TradeNo)
		}
		assert.ElementsMatch(t, check.want, got)
	}
}
