package email

import (
	"fmt"
	"net/smtp"
	"strings"

	"legal-bench-api/internal/config"
)

type Mailer struct {
	host     string
	port     string
	username string
	password string
	from     string
}

func NewMailer(cfg config.Config) Mailer {
	return Mailer{
		host:     cfg.SMTPHost,
		port:     cfg.SMTPPort,
		username: cfg.SMTPUsername,
		password: cfg.SMTPPassword,
		from:     cfg.EmailFrom,
	}
}

func (m Mailer) Configured() bool {
	return m.host != "" && m.port != "" && m.from != ""
}

func (m Mailer) SendActivation(to string, name string, activationURL string) error {
	if !m.Configured() {
		return nil
	}

	subject := "Activate your Legal Bench account"
	body := fmt.Sprintf("Hi %s,\n\nYou've been invited to Legal Bench. Activate your account here:\n\n%s\n\nThis link expires in 7 days.\n", name, activationURL)
	message := strings.Join([]string{
		"From: " + m.from,
		"To: " + to,
		"Subject: " + subject,
		"MIME-Version: 1.0",
		"Content-Type: text/plain; charset=UTF-8",
		"",
		body,
	}, "\r\n")

	addr := m.host + ":" + m.port
	var auth smtp.Auth
	if m.username != "" || m.password != "" {
		auth = smtp.PlainAuth("", m.username, m.password, m.host)
	}

	return smtp.SendMail(addr, auth, m.from, []string{to}, []byte(message))
}
