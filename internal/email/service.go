package email

import (
	"bytes"
	"fmt"
	"log/slog"
	"net/smtp"
)

type Config struct {
	Host     string
	Port     int
	User     string
	Password string
	From     string
}

type Service struct {
	cfg  Config
	auth smtp.Auth
}

func New(cfg Config) *Service {
	var auth smtp.Auth
	if cfg.User != "" && cfg.Password != "" {
		auth = smtp.PlainAuth("", cfg.User, cfg.Password, cfg.Host)
	}
	return &Service{cfg: cfg, auth: auth}
}

func (s *Service) isEnabled() bool {
	return s.cfg.Host != "" && s.auth != nil
}

func (s *Service) SendVerificationEmail(to, token, baseURL string) error {
	if !s.isEnabled() {
		slog.Info("email not configured, skipping verification email", "to", to)
		return nil
	}
	subject := "Verify your email"
	link := fmt.Sprintf("%s/verify-email?token=%s", baseURL, token)
	body := fmt.Sprintf(verifyEmailTemplate, link, token)
	return s.send(to, subject, body)
}

func (s *Service) SendPasswordResetEmail(to, token, baseURL string) error {
	if !s.isEnabled() {
		slog.Info("email not configured, skipping password reset email", "to", to)
		return nil
	}
	subject := "Reset your password"
	link := fmt.Sprintf("%s/reset-password?token=%s", baseURL, token)
	body := fmt.Sprintf(resetPasswordTemplate, link, token)
	return s.send(to, subject, body)
}

func (s *Service) send(to, subject, htmlBody string) error {
	var buf bytes.Buffer
	fmt.Fprintf(&buf, "From: %s\r\n", s.cfg.From)
	fmt.Fprintf(&buf, "To: %s\r\n", to)
	fmt.Fprintf(&buf, "Subject: %s\r\n", subject)
	fmt.Fprintf(&buf, "MIME-Version: 1.0\r\n")
	fmt.Fprintf(&buf, "Content-Type: text/html; charset=\"UTF-8\"\r\n")
	fmt.Fprintf(&buf, "\r\n%s", htmlBody)

	addr := fmt.Sprintf("%s:%d", s.cfg.Host, s.cfg.Port)
	return smtp.SendMail(addr, s.auth, s.cfg.From, []string{to}, buf.Bytes())
}

var verifyEmailTemplate = `<!DOCTYPE html>
<html><body style="font-family:sans-serif;max-width:600px;margin:0 auto;padding:20px">
<h2>Email Verification</h2>
<p>Click the link below to verify your email address:</p>
<p><a href="%s" style="display:inline-block;padding:12px 24px;background:#1890ff;color:#fff;text-decoration:none;border-radius:6px">Verify Email</a></p>
<p>Or copy this link: %s</p>
<p>This link expires in 24 hours.</p>
</body></html>`

var resetPasswordTemplate = `<!DOCTYPE html>
<html><body style="font-family:sans-serif;max-width:600px;margin:0 auto;padding:20px">
<h2>Password Reset</h2>
<p>Click the link below to reset your password:</p>
<p><a href="%s" style="display:inline-block;padding:12px 24px;background:#ff4d4f;color:#fff;text-decoration:none;border-radius:6px">Reset Password</a></p>
<p>Or copy this link: %s</p>
<p>This link expires in 1 hour.</p>
</body></html>`
