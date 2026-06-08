package email

import (
	"fmt"
	"net/smtp"
	"strings"
)

type Mailer struct {
	host string
	port string
	user string
	pass string
	from string
}

func New(host, port, user, pass, from string) *Mailer {
	return &Mailer{host: host, port: port, user: user, pass: pass, from: from}
}

func (m *Mailer) Send(to, subject, body string) error {
	if m.host == "" || m.user == "" {
		return nil // email not configured, skip silently
	}

	auth := smtp.PlainAuth("", m.user, m.pass, m.host)
	msg := strings.Join([]string{
		"From: " + m.from,
		"To: " + to,
		"Subject: " + subject,
		"MIME-Version: 1.0",
		"Content-Type: text/html; charset=UTF-8",
		"",
		body,
	}, "\r\n")

	addr := fmt.Sprintf("%s:%s", m.host, m.port)
	return smtp.SendMail(addr, auth, m.from, []string{to}, []byte(msg))
}

func (m *Mailer) SendVerification(to, verifyURL string) error {
	body := fmt.Sprintf(`
<p>Thank you for registering with <strong>NazScentsation</strong>.</p>
<p>Please confirm your email address by clicking the button below:</p>
<p style="margin:24px 0">
  <a href="%s" style="background:#C9A84C;color:#0a0a0a;padding:12px 28px;text-decoration:none;font-weight:600;border-radius:2px">Verify My Email</a>
</p>
<p>This link expires in 24 hours. If you did not create an account, you can ignore this email.</p>
<br><p style="color:#888;font-size:12px">NazScentsation · Luxury Fragrance Artistry</p>
`, verifyURL)
	return m.Send(to, "Verify your NazScentsation account", body)
}

func (m *Mailer) SendPasswordReset(to, resetURL string) error {
	body := fmt.Sprintf(`
<p>We received a request to reset your <strong>NazScentsation</strong> password.</p>
<p style="margin:24px 0">
  <a href="%s" style="background:#C9A84C;color:#0a0a0a;padding:12px 28px;text-decoration:none;font-weight:600;border-radius:2px">Reset My Password</a>
</p>
<p>This link expires in 1 hour. If you did not request a password reset, you can ignore this email.</p>
<br><p style="color:#888;font-size:12px">NazScentsation · Luxury Fragrance Artistry</p>
`, resetURL)
	return m.Send(to, "Reset your NazScentsation password", body)
}

func (m *Mailer) SendTicketCreated(adminEmail, userEmail, subject string, ticketID int64) error {
	body := fmt.Sprintf(`
<p>A new support ticket has been submitted.</p>
<p><strong>Subject:</strong> %s</p>
<p><strong>From:</strong> %s</p>
<p><strong>Ticket #:</strong> %d</p>
<p>Please log in to your admin panel to review and respond.</p>
<br><p style="color:#888;font-size:12px">NazScentsation Support System</p>
`, subject, userEmail, ticketID)
	return m.Send(adminEmail, fmt.Sprintf("New Support Ticket #%d: %s", ticketID, subject), body)
}

func (m *Mailer) SendOrderStatusUpdate(to, firstName string, orderID int64, status string, total float64) error {
	statusLabel := map[string]string{
		"pending":   "Pending",
		"paid":      "Payment Confirmed",
		"shipped":   "Shipped",
		"delivered": "Delivered",
		"cancelled": "Cancelled",
	}[status]
	if statusLabel == "" {
		statusLabel = status
	}
	greeting := "Hello"
	if firstName != "" {
		greeting = "Hi " + firstName
	}
	body := fmt.Sprintf(`
<p>%s,</p>
<p>Your <strong>Nazscentsation</strong> order <strong>#%d</strong> has been updated.</p>
<table style="border-collapse:collapse;margin:16px 0">
  <tr><td style="padding:6px 12px 6px 0;color:#888;font-size:13px">Order</td><td style="padding:6px 0;font-weight:600">#%d</td></tr>
  <tr><td style="padding:6px 12px 6px 0;color:#888;font-size:13px">Status</td><td style="padding:6px 0;font-weight:600;color:#C9A84C">%s</td></tr>
  <tr><td style="padding:6px 12px 6px 0;color:#888;font-size:13px">Total</td><td style="padding:6px 0">₦%.2f</td></tr>
</table>
<p>Thank you for shopping with us.</p>
<br><p style="color:#888;font-size:12px">Nazscentsation · Luxury Fragrance Artistry</p>
`, greeting, orderID, orderID, statusLabel, total)
	return m.Send(to, fmt.Sprintf("Order #%d Update: %s", orderID, statusLabel), body)
}

func (m *Mailer) SendTicketReply(userEmail, subject string, ticketID int64, replyBody string) error {
	body := fmt.Sprintf(`
<p>Your support ticket has received a reply.</p>
<p><strong>Ticket #%d:</strong> %s</p>
<hr>
<p>%s</p>
<hr>
<p>Log in to your account to view the full conversation and respond.</p>
<br><p style="color:#888;font-size:12px">NazScentsation Support System</p>
`, ticketID, subject, replyBody)
	return m.Send(userEmail, fmt.Sprintf("Reply to your ticket #%d: %s", ticketID, subject), body)
}
