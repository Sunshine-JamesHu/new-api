package common

import (
	"bytes"
	"crypto/tls"
	"encoding/base64"
	"fmt"
	"mime"
	"mime/multipart"
	"net/smtp"
	"net/textproto"
	"path/filepath"
	"slices"
	"strings"
	"time"
	"unicode"
)

func generateMessageID() (string, error) {
	split := strings.Split(SMTPFrom, "@")
	if len(split) < 2 {
		return "", fmt.Errorf("invalid SMTP account")
	}
	domain := strings.Split(SMTPFrom, "@")[1]
	return fmt.Sprintf("<%d.%s@%s>", time.Now().UnixNano(), GetRandomString(12), domain), nil
}

func shouldUseSMTPLoginAuth() bool {
	if SMTPForceAuthLogin {
		return true
	}
	return isOutlookServer(SMTPAccount) || slices.Contains(EmailLoginAuthServerList, SMTPServer)
}

func getSMTPAuth() smtp.Auth {
	return AutoSMTPAuth(SMTPAccount, SMTPToken)
}

func shouldAuthenticateSMTP() bool {
	return SMTPAccount != "" && SMTPToken != ""
}

func smtpTLSConfig() *tls.Config {
	return &tls.Config{
		ServerName:         SMTPServer,
		InsecureSkipVerify: SMTPInsecureSkipVerify, // #nosec G402 -- admin-controlled SMTP compatibility option.
	}
}

func newSMTPClient(addr string) (*smtp.Client, error) {
	if SMTPSSLEnabled || (SMTPPort == 465 && !SMTPStartTLSEnabled) {
		conn, err := tls.Dial("tcp", addr, smtpTLSConfig())
		if err != nil {
			return nil, err
		}
		client, err := smtp.NewClient(conn, SMTPServer)
		if err != nil {
			_ = conn.Close()
			return nil, err
		}
		return client, nil
	}

	client, err := smtp.Dial(addr)
	if err != nil {
		return nil, err
	}

	if SMTPStartTLSEnabled {
		startTLSSupported, _ := client.Extension("STARTTLS")
		if !startTLSSupported {
			_ = client.Close()
			return nil, fmt.Errorf("SMTP server does not support STARTTLS")
		}
		if err := client.StartTLS(smtpTLSConfig()); err != nil {
			_ = client.Close()
			return nil, err
		}
	}

	return client, nil
}

func SendEmail(subject string, receiver string, content string) error {
	return sendEmailMessage(subject, receiver, []byte(fmt.Sprintf("Content-Type: text/html; charset=UTF-8\r\n\r\n%s\r\n", content)))
}

// SendEmailWithAttachment sends an HTML email with one attachment using the
// same SMTP settings as SendEmail. The payload is kept in memory so callers
// can remove temporary upload files immediately after this function returns.
func SendEmailWithAttachment(subject, receiver, content, fileName, contentType string, attachment []byte) error {
	fileName = sanitizeAttachmentFilename(fileName)
	if fileName == "" || len(attachment) == 0 {
		return fmt.Errorf("email attachment is required")
	}
	if strings.ContainsAny(contentType, "\r\n") {
		return fmt.Errorf("invalid email attachment metadata")
	}
	if strings.TrimSpace(contentType) == "" {
		contentType = "application/octet-stream"
	}
	boundary := "new-api-invoice-" + GetRandomString(20)
	var body bytes.Buffer
	body.WriteString("MIME-Version: 1.0\r\n")
	body.WriteString("Content-Type: multipart/mixed; boundary=\"" + boundary + "\"\r\n\r\n")
	writer := multipart.NewWriter(&body)
	if err := writer.SetBoundary(boundary); err != nil {
		return err
	}
	textHeader := make(textproto.MIMEHeader)
	textHeader.Set("Content-Type", "text/html; charset=UTF-8")
	part, err := writer.CreatePart(textHeader)
	if err != nil {
		return err
	}
	if _, err = part.Write([]byte(content)); err != nil {
		return err
	}
	attachmentHeader := make(textproto.MIMEHeader)
	attachmentHeader.Set("Content-Type", mime.FormatMediaType(contentType, map[string]string{"name": fileName}))
	attachmentHeader.Set("Content-Disposition", mime.FormatMediaType("attachment", map[string]string{"filename": fileName}))
	attachmentHeader.Set("Content-Transfer-Encoding", "base64")
	part, err = writer.CreatePart(attachmentHeader)
	if err != nil {
		return err
	}
	encoded := make([]byte, base64.StdEncoding.EncodedLen(len(attachment)))
	base64.StdEncoding.Encode(encoded, attachment)
	for start := 0; start < len(encoded); start += 76 {
		end := start + 76
		if end > len(encoded) {
			end = len(encoded)
		}
		if _, err = part.Write(encoded[start:end]); err != nil {
			return err
		}
		if _, err = part.Write([]byte("\r\n")); err != nil {
			return err
		}
	}
	if err := writer.Close(); err != nil {
		return err
	}
	return sendEmailMessage(subject, receiver, body.Bytes())
}

func sanitizeAttachmentFilename(name string) string {
	name = strings.ReplaceAll(name, "\\", "/")
	name = filepath.Base(name)
	name = strings.Map(func(r rune) rune {
		if r == '"' || r == '\\' || unicode.IsControl(r) {
			return -1
		}
		return r
	}, name)
	return strings.TrimSpace(name)
}

func sendEmailMessage(subject, receiver string, body []byte) error {
	if strings.ContainsAny(receiver, "\r\n") {
		return fmt.Errorf("invalid email receiver")
	}
	if SMTPFrom == "" { // for compatibility
		SMTPFrom = SMTPAccount
	}
	id, err2 := generateMessageID()
	if err2 != nil {
		return err2
	}
	if SMTPServer == "" && SMTPAccount == "" {
		return fmt.Errorf("SMTP 服务器未配置")
	}
	encodedSubject := fmt.Sprintf("=?UTF-8?B?%s?=", base64.StdEncoding.EncodeToString([]byte(subject)))
	mail := []byte(fmt.Sprintf("To: %s\r\n"+
		"From: %s <%s>\r\n"+
		"Subject: %s\r\n"+
		"Date: %s\r\n"+
		"Message-ID: %s\r\n"+ // 添加 Message-ID 头
		"%s\r\n",
		receiver, SystemName, SMTPFrom, encodedSubject, time.Now().Format(time.RFC1123Z), id, body))
	auth := getSMTPAuth()
	addr := fmt.Sprintf("%s:%d", SMTPServer, SMTPPort)
	to := strings.Split(receiver, ";")
	var err error
	client, err := newSMTPClient(addr)
	if err != nil {
		return err
	}
	defer client.Close()
	if shouldAuthenticateSMTP() {
		if err = client.Auth(auth); err != nil {
			return err
		}
	}
	if err = client.Mail(SMTPFrom); err != nil {
		return err
	}
	for _, receiver := range to {
		if err = client.Rcpt(receiver); err != nil {
			return err
		}
	}
	w, err := client.Data()
	if err != nil {
		return err
	}
	_, err = w.Write(mail)
	if err != nil {
		return err
	}
	err = w.Close()
	if err != nil {
		return err
	}
	err = client.Quit()
	if err != nil {
		SysError(fmt.Sprintf("failed to send email to %s: %v", receiver, err))
	}
	return err
}
