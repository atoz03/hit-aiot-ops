package main

import (
	"bytes"
	"fmt"
	"net/smtp"
	"strings"
)

func sendResetPasswordMail(settings MailSettings, toEmail string, subject string, body string) error {
	host := strings.TrimSpace(settings.SMTPHost)
	port := settings.SMTPPort
	user := strings.TrimSpace(settings.SMTPUser)
	pass := strings.TrimSpace(settings.SMTPPass)
	fromEmail := strings.TrimSpace(settings.FromEmail)
	fromName := strings.TrimSpace(settings.FromName)
	toEmail = strings.TrimSpace(toEmail)

	if host == "" || port <= 0 || fromEmail == "" || toEmail == "" {
		return fmt.Errorf("SMTP 配置不完整")
	}
	if fromName == "" {
		fromName = "HIT-AIOT-OPS团队"
	}
	addr := fmt.Sprintf("%s:%d", host, port)

	var auth smtp.Auth
	if user != "" {
		auth = smtp.PlainAuth("", user, pass, host)
	}

	var msg bytes.Buffer
	msg.WriteString(fmt.Sprintf("From: %s <%s>\r\n", fromName, fromEmail))
	msg.WriteString(fmt.Sprintf("To: %s\r\n", toEmail))
	msg.WriteString(fmt.Sprintf("Subject: %s\r\n", subject))
	msg.WriteString("MIME-Version: 1.0\r\n")
	msg.WriteString("Content-Type: text/plain; charset=UTF-8\r\n")
	msg.WriteString("\r\n")
	msg.WriteString(body)
	msg.WriteString("\r\n")

	return smtp.SendMail(addr, auth, fromEmail, []string{toEmail}, msg.Bytes())
}
