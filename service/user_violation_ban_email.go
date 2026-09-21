package service

import (
	"errors"
	"fmt"
	"html/template"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
)

const (
	ViolationBanDefaultSubject = "账号因多次违规已被封禁"
	ViolationBanDefaultBody    = "尊敬的 {{display_name}}：<br><br>由于您的账号多次触发平台违规响应，在收到劝阻后仍继续尝试，平台已对您的账号进行封禁。<br><br>违规次数：{{violation_count}}<br>封禁阈值：{{ban_threshold}}<br><br>如您认为这是误判，请联系平台管理员。<br><br>本邮件由 {{system_name}} 自动发送，请勿直接回复。"
	EmailTemplateMaxSubject    = 500
	EmailTemplateMaxBody       = 20000
)

type ViolationBanEmailTemplate struct {
	Subject string `json:"subject"`
	Body    string `json:"body"`
}

func CurrentViolationBanEmailTemplate() ViolationBanEmailTemplate {
	result := ViolationBanEmailTemplate{
		Subject: ViolationBanDefaultSubject,
		Body:    ViolationBanDefaultBody,
	}
	common.OptionMapRWMutex.RLock()
	if value := strings.TrimSpace(common.OptionMap["violation_ban_email_subject"]); value != "" {
		result.Subject = value
	}
	if value := strings.TrimSpace(common.OptionMap["violation_ban_email_body"]); value != "" {
		result.Body = value
	}
	common.OptionMapRWMutex.RUnlock()
	return result
}

func ValidateEmailTemplate(value ViolationBanEmailTemplate) error {
	value.Subject = strings.TrimSpace(value.Subject)
	value.Body = strings.TrimSpace(value.Body)
	if value.Subject == "" || value.Body == "" || len(value.Subject) > EmailTemplateMaxSubject || len(value.Body) > EmailTemplateMaxBody || strings.ContainsAny(value.Subject, "\r\n") {
		return errors.New("email subject and body are required")
	}
	return nil
}

func RenderViolationBanEmailTemplate(raw ViolationBanEmailTemplate, user *model.User, count int64, threshold int64) (string, string, error) {
	if user == nil {
		return "", "", errors.New("user not found")
	}
	if strings.TrimSpace(raw.Subject) == "" {
		raw.Subject = ViolationBanDefaultSubject
	}
	if strings.TrimSpace(raw.Body) == "" {
		raw.Body = ViolationBanDefaultBody
	}
	displayName := user.DisplayName
	if strings.TrimSpace(displayName) == "" {
		displayName = user.Username
	}
	data := map[string]string{
		"username":        template.HTMLEscapeString(user.Username),
		"display_name":    template.HTMLEscapeString(displayName),
		"email":           template.HTMLEscapeString(user.Email),
		"violation_count": fmt.Sprintf("%d", count),
		"ban_threshold":   fmt.Sprintf("%d", threshold),
		"system_name":     template.HTMLEscapeString(common.SystemName),
	}
	replace := func(input string) string {
		for key, value := range data {
			input = strings.ReplaceAll(input, "{{"+key+"}}", value)
		}
		return input
	}
	return replace(raw.Subject), replace(raw.Body), nil
}

func sendViolationBanEmail(user *model.User, count int64, threshold int64) {
	if user == nil || strings.TrimSpace(user.Email) == "" {
		return
	}
	templateValue := CurrentViolationBanEmailTemplate()
	subject, body, err := RenderViolationBanEmailTemplate(templateValue, user, count, threshold)
	if err != nil {
		common.SysError(fmt.Sprintf("failed to render violation ban email for user %d: %v", user.Id, err))
		return
	}
	if err := common.SendEmail(subject, user.Email, body); err != nil {
		common.SysError(fmt.Sprintf("failed to send violation ban email for user %d: %v", user.Id, err))
	}
}
