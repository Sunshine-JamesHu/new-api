package setting

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNormalizeAlipayPaymentMode(t *testing.T) {
	t.Parallel()

	assert.Equal(t, AlipayPaymentModeAuto, NormalizeAlipayPaymentMode(""))
	assert.Equal(t, AlipayPaymentModeAuto, NormalizeAlipayPaymentMode("qrcode"))
	assert.Equal(t, AlipayPaymentModeRedirect, NormalizeAlipayPaymentMode(" redirect "))
	assert.Equal(t, AlipayPaymentModeRedirect, NormalizeAlipayPaymentMode("REDIRECT"))
}
