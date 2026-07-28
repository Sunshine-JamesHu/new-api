package controller

import (
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTopUpFilterContext(t *testing.T, query string) *gin.Context {
	t.Helper()
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	ctx.Request = httptest.NewRequest("GET", "/api/user/topup?"+query, nil)
	return ctx
}

func TestParseTopUpFilterUserScopeIgnoresUserID(t *testing.T) {
	filter, err := parseTopUpFilter(
		newTopUpFilterContext(t, "keyword=order-1&invoice_status=unissued&user_id=42"),
		false,
	)

	require.NoError(t, err)
	assert.Equal(t, "order-1", filter.Keyword)
	require.NotNil(t, filter.InvoiceIssued)
	assert.False(t, *filter.InvoiceIssued)
	assert.Nil(t, filter.UserId)
}

func TestParseTopUpFilterAdminScopeValidatesUserID(t *testing.T) {
	filter, err := parseTopUpFilter(
		newTopUpFilterContext(t, "invoice_status=issued&user_id=42"),
		true,
	)

	require.NoError(t, err)
	require.NotNil(t, filter.InvoiceIssued)
	assert.True(t, *filter.InvoiceIssued)
	require.NotNil(t, filter.UserId)
	assert.Equal(t, 42, *filter.UserId)

	_, err = parseTopUpFilter(
		newTopUpFilterContext(t, "invoice_status=all&user_id=0"),
		true,
	)
	require.Error(t, err)
}

func TestParseTopUpFilterRejectsInvalidInvoiceStatus(t *testing.T) {
	_, err := parseTopUpFilter(
		newTopUpFilterContext(t, "invoice_status=unknown"),
		false,
	)

	require.Error(t, err)
}
