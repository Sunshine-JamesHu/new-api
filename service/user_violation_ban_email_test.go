package service

import (
	"testing"

	"github.com/QuantumNous/new-api/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRenderViolationBanEmailTemplateReplacesAndEscapesUserValues(t *testing.T) {
	user := &model.User{
		Username:    "<user>",
		DisplayName: "<Display & Name>",
		Email:       "user@example.com",
	}
	raw := ViolationBanEmailTemplate{
		Subject: "Ban {{username}} {{violation_count}}",
		Body:    "Hello {{display_name}} ({{email}}), threshold={{ban_threshold}}/{{system_name}}",
	}

	subject, body, err := RenderViolationBanEmailTemplate(raw, user, 3, 3)

	require.NoError(t, err)
	assert.Equal(t, "Ban &lt;user&gt; 3", subject)
	assert.Contains(t, body, "Hello &lt;Display &amp; Name&gt; (user@example.com)")
	assert.Contains(t, body, "threshold=3/")
}

func TestRenderViolationBanEmailTemplateUsesUsernameWhenDisplayNameIsBlank(t *testing.T) {
	user := &model.User{Username: "alice", Email: "alice@example.com"}

	_, body, err := RenderViolationBanEmailTemplate(
		ViolationBanEmailTemplate{Body: "{{display_name}}"},
		user,
		1,
		2,
	)

	require.NoError(t, err)
	assert.Equal(t, "alice", body)
}

func TestValidateEmailTemplateRejectsSubjectHeaderInjection(t *testing.T) {
	err := ValidateEmailTemplate(ViolationBanEmailTemplate{
		Subject: "subject\r\nBcc: attacker@example.com",
		Body:    "body",
	})

	require.Error(t, err)
}

func TestValidateEmailTemplateRejectsOversizedBody(t *testing.T) {
	err := ValidateEmailTemplate(ViolationBanEmailTemplate{
		Subject: "subject",
		Body:    string(make([]byte, EmailTemplateMaxBody+1)),
	})

	require.Error(t, err)
}
