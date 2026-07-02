package controller

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting"
	"github.com/QuantumNous/new-api/setting/operation_setting"

	"github.com/gin-gonic/gin"
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

func TestAlipayNotifyCreditsMatchingSignedNotificationAndIsIdempotent(t *testing.T) {
	setupAlipayNotifyTest(t)
	tradeNo := insertAlipayNotifyTopUp(t, 201, 2, 9.99)
	values := signedAlipayNotifyValues(t, tradeNo, "9.99", setting.AlipayAppId)

	recorder := postAlipayNotify(t, values)
	require.Equal(t, http.StatusOK, recorder.Code)
	assert.Equal(t, "success", recorder.Body.String())
	assert.Equal(t, common.TopUpStatusSuccess, getAlipayNotifyTopUpStatus(t, tradeNo))
	assert.Equal(t, 2000, getAlipayNotifyUserQuota(t, 201))

	recorder = postAlipayNotify(t, values)
	require.Equal(t, http.StatusOK, recorder.Code)
	assert.Equal(t, "success", recorder.Body.String())
	assert.Equal(t, 2000, getAlipayNotifyUserQuota(t, 201))
}

func TestAlipayNotifyRejectsSignedNotificationWithWrongAmount(t *testing.T) {
	setupAlipayNotifyTest(t)
	tradeNo := insertAlipayNotifyTopUp(t, 202, 2, 9.99)
	values := signedAlipayNotifyValues(t, tradeNo, "0.01", setting.AlipayAppId)

	recorder := postAlipayNotify(t, values)
	require.Equal(t, http.StatusOK, recorder.Code)
	assert.Equal(t, "fail", recorder.Body.String())
	assert.Equal(t, common.TopUpStatusPending, getAlipayNotifyTopUpStatus(t, tradeNo))
	assert.Equal(t, 0, getAlipayNotifyUserQuota(t, 202))
}

func TestAlipayNotifyRejectsSignedNotificationWithWrongAppID(t *testing.T) {
	setupAlipayNotifyTest(t)
	tradeNo := insertAlipayNotifyTopUp(t, 203, 2, 9.99)
	values := signedAlipayNotifyValues(t, tradeNo, "9.99", "2021000000000999")

	recorder := postAlipayNotify(t, values)
	require.Equal(t, http.StatusOK, recorder.Code)
	assert.Equal(t, "fail", recorder.Body.String())
	assert.Equal(t, common.TopUpStatusPending, getAlipayNotifyTopUpStatus(t, tradeNo))
	assert.Equal(t, 0, getAlipayNotifyUserQuota(t, 203))
}

func setupAlipayNotifyTest(t *testing.T) {
	t.Helper()
	setupModelListControllerTestDB(t)
	require.NoError(t, model.DB.AutoMigrate(&model.TopUp{}, &model.Log{}, &model.AffiliateRebate{}))

	originalAppID := setting.AlipayAppId
	originalPrivateKey := setting.AlipayPrivateKey
	originalPublicKey := setting.AlipayPublicKey
	originalClient := cachedAlipayClient
	originalClientSignature := cachedAlipayClientSignature
	originalQuotaPerUnit := common.QuotaPerUnit
	originalPaymentSetting := *operation_setting.GetPaymentSetting()
	t.Cleanup(func() {
		setting.AlipayAppId = originalAppID
		setting.AlipayPrivateKey = originalPrivateKey
		setting.AlipayPublicKey = originalPublicKey
		cachedAlipayClient = originalClient
		cachedAlipayClientSignature = originalClientSignature
		common.QuotaPerUnit = originalQuotaPerUnit
		*operation_setting.GetPaymentSetting() = originalPaymentSetting
	})

	appPrivateKey, appPublicKey := generateAlipayNotifyRSAKeys(t)
	alipayPrivateKey, alipayPublicKey := generateAlipayNotifyRSAKeys(t)
	setting.AlipayAppId = "2021000000000000"
	setting.AlipayPrivateKey = appPrivateKey
	setting.AlipayPublicKey = alipayPublicKey
	cachedAlipayClient = nil
	cachedAlipayClientSignature = ""
	common.QuotaPerUnit = 1000
	operation_setting.GetPaymentSetting().ComplianceConfirmed = true
	operation_setting.GetPaymentSetting().ComplianceTermsVersion = operation_setting.CurrentComplianceTermsVersion
	_ = appPublicKey

	alipayNotifySigningPrivateKey = alipayPrivateKey
}

var alipayNotifySigningPrivateKey string

func generateAlipayNotifyRSAKeys(t *testing.T) (privateKey string, publicKey string) {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)

	privateBlock := &pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(key),
	}
	publicBytes, err := x509.MarshalPKIXPublicKey(&key.PublicKey)
	require.NoError(t, err)
	publicBlock := &pem.Block{
		Type:  "PUBLIC KEY",
		Bytes: publicBytes,
	}
	return string(pem.EncodeToMemory(privateBlock)), string(pem.EncodeToMemory(publicBlock))
}

func insertAlipayNotifyTopUp(t *testing.T, userID int, amount int64, money float64) string {
	t.Helper()
	require.NoError(t, model.DB.Create(&model.User{
		Id:       userID,
		Username: fmt.Sprintf("alipay_notify_user_%d", userID),
		Status:   common.UserStatusEnabled,
	}).Error)

	tradeNo := fmt.Sprintf("ALIPAY_NOTIFY_%d", userID)
	require.NoError(t, (&model.TopUp{
		UserId:          userID,
		Amount:          amount,
		Money:           money,
		TradeNo:         tradeNo,
		PaymentMethod:   model.PaymentMethodAlipay,
		PaymentProvider: model.PaymentProviderAlipay,
		CreateTime:      time.Now().Unix(),
		Status:          common.TopUpStatusPending,
	}).Insert())
	return tradeNo
}

func signedAlipayNotifyValues(t *testing.T, tradeNo string, totalAmount string, appID string) url.Values {
	t.Helper()
	client, err := alipay.New(appID, alipayNotifySigningPrivateKey, true)
	require.NoError(t, err)

	values := url.Values{}
	values.Set("app_id", appID)
	values.Set("auth_app_id", appID)
	values.Set("charset", "utf-8")
	values.Set("notify_id", "notify-"+tradeNo)
	values.Set("notify_time", "2026-07-02 23:59:59")
	values.Set("notify_type", "trade_status_sync")
	values.Set("out_trade_no", tradeNo)
	values.Set("seller_id", "2088000000000000")
	values.Set("sign_type", "RSA2")
	values.Set("total_amount", totalAmount)
	values.Set("trade_no", "2026070222000000000000000000")
	values.Set("trade_status", "TRADE_SUCCESS")
	values.Set("version", "1.0")

	signType := values.Get("sign_type")
	values.Del("sign_type")
	signature, err := client.SignValues(values)
	require.NoError(t, err)
	values.Set("sign_type", signType)
	values.Set("sign", base64.StdEncoding.EncodeToString(signature))
	return values
}

func postAlipayNotify(t *testing.T, values url.Values) *httptest.ResponseRecorder {
	t.Helper()
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/alipay/notify", bytes.NewBufferString(values.Encode()))
	c.Request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	AlipayNotify(c)
	return recorder
}

func getAlipayNotifyTopUpStatus(t *testing.T, tradeNo string) string {
	t.Helper()
	topUp := model.GetTopUpByTradeNo(tradeNo)
	require.NotNil(t, topUp)
	return topUp.Status
}

func getAlipayNotifyUserQuota(t *testing.T, userID int) int {
	t.Helper()
	var user model.User
	require.NoError(t, model.DB.Select("quota").Where("id = ?", userID).First(&user).Error)
	return user.Quota
}
