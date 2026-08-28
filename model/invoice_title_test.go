package model

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestInvoiceTitleCRUDNormalizesAndScopesByUser(t *testing.T) {
	truncateTables(t)
	title := &InvoiceTitle{
		UserID: 1001,
		Name:   "  Example Company  ", TaxNumber: "  TAX-001  ",
		Address: "  Example address  ", Email: "  Invoice@Example.com  ",
	}
	require.NoError(t, CreateInvoiceTitle(title))
	assert.Equal(t, "Example Company", title.Name)
	assert.Equal(t, "TAX-001", title.TaxNumber)
	assert.Equal(t, "Example address", title.Address)
	assert.Equal(t, "invoice@example.com", title.Email)

	titles, err := ListInvoiceTitles(1001)
	require.NoError(t, err)
	require.Len(t, titles, 1)

	title.Name = "Updated Company"
	title.Email = "updated@example.com"
	require.NoError(t, UpdateInvoiceTitle(title))
	updated, err := GetInvoiceTitleForUser(nil, 1001, title.ID)
	require.NoError(t, err)
	assert.Equal(t, "Updated Company", updated.Name)
	assert.Equal(t, "updated@example.com", updated.Email)

	_, err = GetInvoiceTitleForUser(nil, 1002, title.ID)
	require.ErrorIs(t, err, ErrInvoiceTitleNotFound)
	require.ErrorIs(t, DeleteInvoiceTitle(1002, title.ID), ErrInvoiceTitleNotFound)
	require.NoError(t, DeleteInvoiceTitle(1001, title.ID))
	require.ErrorIs(t, DeleteInvoiceTitle(1001, title.ID), ErrInvoiceTitleNotFound)
}

func TestInvoiceTitleValidationRejectsInvalidFields(t *testing.T) {
	for _, title := range []*InvoiceTitle{
		{UserID: 1, Name: "", TaxNumber: "tax", Address: "address", Email: "user@example.com"},
		{UserID: 1, Name: "name", TaxNumber: "tax", Address: "address", Email: "invalid"},
		{UserID: 1, Name: "name", TaxNumber: "tax", Address: "address", Email: "user@example.com\r\nBcc:bad@example.com"},
	} {
		assert.ErrorIs(t, title.Normalize(), ErrInvoiceTitleInvalid)
	}
}
