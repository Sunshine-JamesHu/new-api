package setting

import (
	"strings"
	"sync"
)

const (
	AlipayPaymentModeAuto     = "auto"
	AlipayPaymentModeRedirect = "redirect"
)

var (
	AlipayAppId        = ""
	AlipayPrivateKey   = ""
	AlipayPublicKey    = ""
	AlipayReturnUrl    = ""
	AlipayPaymentMode  = AlipayPaymentModeAuto
	AlipayClientRWLock sync.RWMutex
)

func NormalizeAlipayPaymentMode(mode string) string {
	switch strings.ToLower(strings.TrimSpace(mode)) {
	case AlipayPaymentModeRedirect:
		return AlipayPaymentModeRedirect
	default:
		return AlipayPaymentModeAuto
	}
}
