package controller

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting"
	"github.com/QuantumNous/new-api/setting/operation_setting"

	"github.com/gin-gonic/gin"
	"github.com/shopspring/decimal"
	"github.com/smartwalle/alipay/v3"
)

const (
	alipayProductCodePreCreate = "FACE_TO_FACE_PAYMENT"
	alipayProductCodeWapPay    = "QUICK_WAP_WAY"
	alipayProductCodePagePay   = "FAST_INSTANT_TRADE_PAY"
	alipayTradeStatusSuccess   = "TRADE_SUCCESS"
	alipayTradeStatusFinished  = "TRADE_FINISHED"
)

type AlipayRequest struct {
	Amount   int64 `json:"amount"`
	IsMobile bool  `json:"is_mobile"`
}

type alipayPaymentResult struct {
	PayURL string `json:"pay_url,omitempty"`
	QRCode string `json:"qr_code,omitempty"`
}

var cachedAlipayClient *alipay.Client
var cachedAlipayClientSignature string

var (
	alipayTradeWapPay = func(client *alipay.Client, param alipay.TradeWapPay) (*url.URL, error) {
		return client.TradeWapPay(param)
	}
	alipayTradePreCreate = func(ctx context.Context, client *alipay.Client, param alipay.TradePreCreate) (*alipay.TradePreCreateRsp, error) {
		return client.TradePreCreate(ctx, param)
	}
	alipayTradePagePay = func(client *alipay.Client, param alipay.TradePagePay) (*url.URL, error) {
		return client.TradePagePay(param)
	}
)

func getAlipayClient() (*alipay.Client, error) {
	signature := strings.Join([]string{
		strings.TrimSpace(setting.AlipayAppId),
		strings.TrimSpace(setting.AlipayPrivateKey),
		strings.TrimSpace(setting.AlipayPublicKey),
	}, "\x00")

	setting.AlipayClientRWLock.RLock()
	if cachedAlipayClient != nil && cachedAlipayClientSignature == signature {
		client := cachedAlipayClient
		setting.AlipayClientRWLock.RUnlock()
		return client, nil
	}
	setting.AlipayClientRWLock.RUnlock()

	setting.AlipayClientRWLock.Lock()
	defer setting.AlipayClientRWLock.Unlock()
	if cachedAlipayClient != nil && cachedAlipayClientSignature == signature {
		return cachedAlipayClient, nil
	}

	appID := strings.TrimSpace(setting.AlipayAppId)
	privateKey := strings.TrimSpace(setting.AlipayPrivateKey)
	publicKey := strings.TrimSpace(setting.AlipayPublicKey)
	if appID == "" || privateKey == "" || publicKey == "" {
		return nil, fmt.Errorf("alipay config is incomplete")
	}

	client, err := alipay.New(appID, privateKey, true)
	if err != nil {
		return nil, fmt.Errorf("init alipay client: %w", err)
	}
	if err := client.LoadAliPayPublicKey(publicKey); err != nil {
		return nil, fmt.Errorf("load alipay public key: %w", err)
	}
	cachedAlipayClient = client
	cachedAlipayClientSignature = signature
	return cachedAlipayClient, nil
}

func RequestAlipayAmount(c *gin.Context) {
	RequestAmount(c)
}

func RequestAlipayPay(c *gin.Context) {
	var req AlipayRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusOK, gin.H{"message": "error", "data": "invalid parameters"})
		return
	}
	if req.Amount < getMinTopup() {
		c.JSON(http.StatusOK, gin.H{"message": "error", "data": fmt.Sprintf("top-up amount cannot be less than %d", getMinTopup())})
		return
	}
	if !isAlipayTopUpEnabled() {
		c.JSON(http.StatusOK, gin.H{"message": "error", "data": "official Alipay payment is not configured"})
		return
	}

	id := c.GetInt("id")
	group, err := model.GetUserGroup(id, true)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"message": "error", "data": "failed to get user group"})
		return
	}
	payMoney := getPayMoney(req.Amount, group)
	if payMoney < 0.01 {
		c.JSON(http.StatusOK, gin.H{"message": "error", "data": "top-up payment amount is too low"})
		return
	}

	client, err := getAlipayClient()
	if err != nil {
		logger.LogError(c.Request.Context(), fmt.Sprintf("official alipay client init failed user_id=%d error=%q", id, err.Error()))
		c.JSON(http.StatusOK, gin.H{"message": "error", "data": "official Alipay payment is not configured"})
		return
	}

	tradeNo := fmt.Sprintf("USR%dALIPAY%s%d", id, common.GetRandomString(6), time.Now().Unix())
	callBackAddress := service.GetCallbackAddress()
	notifyURL := callBackAddress + "/api/alipay/notify"
	returnURL := strings.TrimSpace(setting.AlipayReturnUrl)
	if returnURL == "" {
		returnURL = callBackAddress + paymentReturnPath("/console/log")
	}

	result, err := createAlipayPayment(c.Request.Context(), client, req.IsMobile, tradeNo, req.Amount, payMoney, notifyURL, returnURL)
	if err != nil {
		logger.LogError(c.Request.Context(), fmt.Sprintf("official alipay create payment failed user_id=%d trade_no=%s amount=%d error=%q", id, tradeNo, req.Amount, err.Error()))
		c.JSON(http.StatusOK, gin.H{"message": "error", "data": "failed to start Alipay payment"})
		return
	}

	amount := req.Amount
	if operation_setting.GetQuotaDisplayType() == operation_setting.QuotaDisplayTypeTokens {
		dAmount := decimal.NewFromInt(amount)
		dQuotaPerUnit := decimal.NewFromFloat(common.QuotaPerUnit)
		amount = dAmount.Div(dQuotaPerUnit).IntPart()
	}
	topUp := &model.TopUp{
		UserId:          id,
		Amount:          amount,
		Money:           payMoney,
		TradeNo:         tradeNo,
		PaymentMethod:   model.PaymentMethodAlipay,
		PaymentProvider: model.PaymentProviderAlipay,
		CreateTime:      time.Now().Unix(),
		Status:          common.TopUpStatusPending,
	}
	if err := topUp.Insert(); err != nil {
		logger.LogError(c.Request.Context(), fmt.Sprintf("official alipay create topup failed user_id=%d trade_no=%s amount=%d error=%q", id, tradeNo, req.Amount, err.Error()))
		c.JSON(http.StatusOK, gin.H{"message": "error", "data": "failed to create order"})
		return
	}

	logger.LogInfo(c.Request.Context(), fmt.Sprintf("official alipay topup created user_id=%d trade_no=%s amount=%d money=%.2f result=%q", id, tradeNo, req.Amount, payMoney, common.GetJsonString(result)))
	common.ApiSuccess(c, gin.H{
		"trade_no": tradeNo,
		"pay_url":  result.PayURL,
		"qr_code":  result.QRCode,
	})
}

func createAlipayPayment(ctx context.Context, client *alipay.Client, isMobile bool, tradeNo string, amount int64, payMoney float64, notifyURL string, returnURL string) (*alipayPaymentResult, error) {
	subject := fmt.Sprintf("TUC%d", amount)
	money := strconv.FormatFloat(payMoney, 'f', 2, 64)
	if isMobile {
		param := alipay.TradeWapPay{}
		param.OutTradeNo = tradeNo
		param.TotalAmount = money
		param.Subject = subject
		param.ProductCode = alipayProductCodeWapPay
		param.NotifyURL = notifyURL
		param.ReturnURL = returnURL
		payURL, err := alipayTradeWapPay(client, param)
		if err != nil {
			return nil, fmt.Errorf("alipay wap pay: %w", err)
		}
		return &alipayPaymentResult{PayURL: payURL.String()}, nil
	}

	if setting.NormalizeAlipayPaymentMode(setting.AlipayPaymentMode) != setting.AlipayPaymentModeRedirect {
		param := alipay.TradePreCreate{}
		param.OutTradeNo = tradeNo
		param.TotalAmount = money
		param.Subject = subject
		param.ProductCode = alipayProductCodePreCreate
		param.NotifyURL = notifyURL
		rsp, err := alipayTradePreCreate(ctx, client, param)
		if err == nil && rsp != nil && !rsp.IsFailure() && strings.TrimSpace(rsp.QRCode) != "" {
			return &alipayPaymentResult{QRCode: rsp.QRCode}, nil
		}
		if err != nil {
			common.SysLog(fmt.Sprintf("alipay precreate failed, fallback to page pay: %s", err.Error()))
		} else if rsp != nil && rsp.IsFailure() {
			common.SysLog(fmt.Sprintf("alipay precreate failed, fallback to page pay: %s", rsp.Error.Error()))
		}
	}

	param := alipay.TradePagePay{}
	param.OutTradeNo = tradeNo
	param.TotalAmount = money
	param.Subject = subject
	param.ProductCode = alipayProductCodePagePay
	param.NotifyURL = notifyURL
	param.ReturnURL = returnURL
	payURL, err := alipayTradePagePay(client, param)
	if err != nil {
		return nil, fmt.Errorf("alipay page pay: %w", err)
	}
	return &alipayPaymentResult{PayURL: payURL.String()}, nil
}

func AlipayNotify(c *gin.Context) {
	if !isAlipayWebhookEnabled() {
		logger.LogWarn(c.Request.Context(), fmt.Sprintf("official alipay webhook rejected reason=webhook_disabled path=%q client_ip=%s", c.Request.RequestURI, c.ClientIP()))
		_, _ = c.Writer.Write([]byte("fail"))
		return
	}

	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		logger.LogError(c.Request.Context(), fmt.Sprintf("official alipay webhook body read failed path=%q client_ip=%s error=%q", c.Request.RequestURI, c.ClientIP(), err.Error()))
		_, _ = c.Writer.Write([]byte("fail"))
		return
	}

	values, err := url.ParseQuery(string(body))
	if err != nil {
		logger.LogWarn(c.Request.Context(), fmt.Sprintf("official alipay webhook parse failed path=%q client_ip=%s error=%q", c.Request.RequestURI, c.ClientIP(), err.Error()))
		_, _ = c.Writer.Write([]byte("fail"))
		return
	}
	client, err := getAlipayClient()
	if err != nil {
		logger.LogError(c.Request.Context(), fmt.Sprintf("official alipay client not initialized path=%q client_ip=%s error=%q", c.Request.RequestURI, c.ClientIP(), err.Error()))
		_, _ = c.Writer.Write([]byte("fail"))
		return
	}
	notification, err := client.DecodeNotification(c.Request.Context(), values)
	if err != nil {
		logger.LogWarn(c.Request.Context(), fmt.Sprintf("official alipay webhook verify failed path=%q client_ip=%s error=%q", c.Request.RequestURI, c.ClientIP(), err.Error()))
		_, _ = c.Writer.Write([]byte("fail"))
		return
	}

	notifyResponse := "fail"
	defer func() {
		_, _ = c.Writer.Write([]byte(notifyResponse))
	}()

	if notification.TradeStatus != alipayTradeStatusSuccess && notification.TradeStatus != alipayTradeStatusFinished {
		logger.LogInfo(c.Request.Context(), fmt.Sprintf("official alipay webhook ignored trade_no=%s out_trade_no=%s trade_status=%s client_ip=%s", notification.TradeNo, notification.OutTradeNo, notification.TradeStatus, c.ClientIP()))
		notifyResponse = "success"
		return
	}

	LockOrder(notification.OutTradeNo)
	defer UnlockOrder(notification.OutTradeNo)
	if err := model.RechargeAlipay(notification.OutTradeNo, c.ClientIP()); err != nil {
		logger.LogError(c.Request.Context(), fmt.Sprintf("official alipay credit failed trade_no=%s alipay_trade_no=%s client_ip=%s error=%q", notification.OutTradeNo, notification.TradeNo, c.ClientIP(), err.Error()))
		return
	}
	notifyResponse = "success"
	logger.LogInfo(c.Request.Context(), fmt.Sprintf("official alipay credit succeeded trade_no=%s alipay_trade_no=%s client_ip=%s", notification.OutTradeNo, notification.TradeNo, c.ClientIP()))
}
