package controller

import (
	"context"
	"errors"
	"net/url"
	"testing"

	"github.com/QuantumNous/new-api/setting"

	"github.com/smartwalle/alipay/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCreateAlipayPaymentUsesPagePayForDesktopFallback(t *testing.T) {
	originalPreCreate := alipayTradePreCreate
	originalPagePay := alipayTradePagePay
	originalWapPay := alipayTradeWapPay
	originalMode := setting.AlipayPaymentMode
	t.Cleanup(func() {
		alipayTradePreCreate = originalPreCreate
		alipayTradePagePay = originalPagePay
		alipayTradeWapPay = originalWapPay
		setting.AlipayPaymentMode = originalMode
	})

	setting.AlipayPaymentMode = setting.AlipayPaymentModeAuto
	preCreateCalls := 0
	pagePayCalls := 0
	wapPayCalls := 0
	alipayTradePreCreate = func(ctx context.Context, client *alipay.Client, param alipay.TradePreCreate) (*alipay.TradePreCreateRsp, error) {
		preCreateCalls++
		assert.Equal(t, "trade_100", param.OutTradeNo)
		assert.Equal(t, "88.00", param.TotalAmount)
		assert.Equal(t, alipayProductCodePreCreate, param.ProductCode)
		return nil, errors.New("face-to-face payment is not enabled")
	}
	alipayTradePagePay = func(client *alipay.Client, param alipay.TradePagePay) (*url.URL, error) {
		pagePayCalls++
		assert.Equal(t, "trade_100", param.OutTradeNo)
		assert.Equal(t, "88.00", param.TotalAmount)
		assert.Equal(t, alipayProductCodePagePay, param.ProductCode)
		assert.Equal(t, "https://merchant.example.com/api/alipay/notify", param.NotifyURL)
		assert.Equal(t, "https://merchant.example.com/console/log", param.ReturnURL)
		return url.Parse("https://openapi.alipay.com/gateway.do?page-pay")
	}
	alipayTradeWapPay = func(client *alipay.Client, param alipay.TradeWapPay) (*url.URL, error) {
		wapPayCalls++
		return url.Parse("https://openapi.alipay.com/gateway.do?wap-pay")
	}

	result, err := createAlipayPayment(
		context.Background(),
		&alipay.Client{},
		false,
		"trade_100",
		1000,
		88,
		"https://merchant.example.com/api/alipay/notify",
		"https://merchant.example.com/console/log",
	)

	require.NoError(t, err)
	assert.Equal(t, 1, preCreateCalls)
	assert.Equal(t, 1, pagePayCalls)
	assert.Equal(t, 0, wapPayCalls)
	assert.Equal(t, "https://openapi.alipay.com/gateway.do?page-pay", result.PayURL)
	assert.Empty(t, result.QRCode)
}

func TestCreateAlipayPaymentRedirectModeSkipsPrecreate(t *testing.T) {
	originalPreCreate := alipayTradePreCreate
	originalPagePay := alipayTradePagePay
	originalMode := setting.AlipayPaymentMode
	t.Cleanup(func() {
		alipayTradePreCreate = originalPreCreate
		alipayTradePagePay = originalPagePay
		setting.AlipayPaymentMode = originalMode
	})

	setting.AlipayPaymentMode = setting.AlipayPaymentModeRedirect
	preCreateCalls := 0
	pagePayCalls := 0
	alipayTradePreCreate = func(ctx context.Context, client *alipay.Client, param alipay.TradePreCreate) (*alipay.TradePreCreateRsp, error) {
		preCreateCalls++
		return &alipay.TradePreCreateRsp{
			Error:  alipay.Error{Code: alipay.CodeSuccess},
			QRCode: "https://qr.alipay.example.com/precreate-token",
		}, nil
	}
	alipayTradePagePay = func(client *alipay.Client, param alipay.TradePagePay) (*url.URL, error) {
		pagePayCalls++
		assert.Equal(t, alipayProductCodePagePay, param.ProductCode)
		return url.Parse("https://openapi.alipay.com/gateway.do?page-pay")
	}

	result, err := createAlipayPayment(
		context.Background(),
		&alipay.Client{},
		false,
		"trade_101",
		1000,
		66,
		"https://merchant.example.com/api/alipay/notify",
		"https://merchant.example.com/console/log",
	)

	require.NoError(t, err)
	assert.Equal(t, 0, preCreateCalls)
	assert.Equal(t, 1, pagePayCalls)
	assert.Equal(t, "https://openapi.alipay.com/gateway.do?page-pay", result.PayURL)
	assert.Empty(t, result.QRCode)
}

func TestCreateAlipayPaymentUsesPrecreateQrForDesktopWhenAvailable(t *testing.T) {
	originalPreCreate := alipayTradePreCreate
	originalPagePay := alipayTradePagePay
	originalMode := setting.AlipayPaymentMode
	t.Cleanup(func() {
		alipayTradePreCreate = originalPreCreate
		alipayTradePagePay = originalPagePay
		setting.AlipayPaymentMode = originalMode
	})

	setting.AlipayPaymentMode = setting.AlipayPaymentModeAuto
	preCreateCalls := 0
	pagePayCalls := 0
	alipayTradePreCreate = func(ctx context.Context, client *alipay.Client, param alipay.TradePreCreate) (*alipay.TradePreCreateRsp, error) {
		preCreateCalls++
		assert.Equal(t, alipayProductCodePreCreate, param.ProductCode)
		return &alipay.TradePreCreateRsp{
			Error:  alipay.Error{Code: alipay.CodeSuccess},
			QRCode: "https://qr.alipay.example.com/precreate-token",
		}, nil
	}
	alipayTradePagePay = func(client *alipay.Client, param alipay.TradePagePay) (*url.URL, error) {
		pagePayCalls++
		return url.Parse("https://openapi.alipay.com/gateway.do?page-pay")
	}

	result, err := createAlipayPayment(
		context.Background(),
		&alipay.Client{},
		false,
		"trade_102",
		1000,
		66,
		"https://merchant.example.com/api/alipay/notify",
		"https://merchant.example.com/console/log",
	)

	require.NoError(t, err)
	assert.Equal(t, 1, preCreateCalls)
	assert.Equal(t, 0, pagePayCalls)
	assert.Equal(t, "https://qr.alipay.example.com/precreate-token", result.QRCode)
	assert.Empty(t, result.PayURL)
}

func TestCreateAlipayPaymentUsesWapPayForMobile(t *testing.T) {
	originalWapPay := alipayTradeWapPay
	t.Cleanup(func() {
		alipayTradeWapPay = originalWapPay
	})

	wapPayCalls := 0
	alipayTradeWapPay = func(client *alipay.Client, param alipay.TradeWapPay) (*url.URL, error) {
		wapPayCalls++
		assert.Equal(t, "trade_103", param.OutTradeNo)
		assert.Equal(t, alipayProductCodeWapPay, param.ProductCode)
		assert.Equal(t, "https://merchant.example.com/console/log", param.ReturnURL)
		return url.Parse("https://openapi.alipay.com/gateway.do?wap-pay")
	}

	result, err := createAlipayPayment(
		context.Background(),
		&alipay.Client{},
		true,
		"trade_103",
		1000,
		18,
		"https://merchant.example.com/api/alipay/notify",
		"https://merchant.example.com/console/log",
	)

	require.NoError(t, err)
	assert.Equal(t, 1, wapPayCalls)
	assert.Equal(t, "https://openapi.alipay.com/gateway.do?wap-pay", result.PayURL)
	assert.Empty(t, result.QRCode)
}
