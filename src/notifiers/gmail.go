package notifiers

/*
ESSENTIAL PROCESS:
Implements the Gmail notification sender via SMTP with architectural hardening.
Handles both Port 587 (STARTTLS) and Port 465 (Implicit TLS).
Dispatches operational and delivery messages through the ecosystem Universal Logger.

DATA FLOW:
1. Receives message payload, subject, and optional attachment path.
2. Dial the SMTP server using net.Dialer with context support.
3. Upgrades to TLS (if 587) or starts with TLS (if 465).
4. Executes SMTP state machine (Auth -> Mail -> Rcpt -> Data).
5. Respects context deadlines throughout the network operation.
6. Logs dispatch lifecycle and delivery results through Logger.

KEY PARAMETERS:
- from: Sender email address.
- to: Recipient email address.
- smtp: SMTP server address (default: smtp.gmail.com).
- port: SMTP server port (default: 587).
- logger: Unified logger instance for operational messages.
*/

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/base64"
	"fmt"
	"mime/multipart"
	"net"
	"net/smtp"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	log_interfaces "github.com/Bastien-Antigravity/universal-logger/src/interfaces"
)

type GmailSender struct {
	tag      string
	from     string
	to       string
	smtp     string
	port     int
	passwd   string
	logLevel string
	logger   log_interfaces.Logger
	decrypt  func(string) (string, error)
}

// -----------------------------------------------------------------------------

func NewGmailSender(gmailConf map[string]string, confName string, logger log_interfaces.Logger, decrypt func(string) (string, error)) (*GmailSender, error) {
	tag := getOption(gmailConf, "TAG", "tag", "NOTIF_GMAIL_TAG")
	if tag == "" {
		tag = confName
	}

	from := getOption(gmailConf, "FROM", "from", "NOTIF_GMAIL_FROM", "GMAIL_FROM")
	to := getOption(gmailConf, "TO", "to", "NOTIF_GMAIL_TO", "GMAIL_TO")
	passwd := getOption(gmailConf, "PASSWD", "passwd", "PASSWORD", "password", "NOTIF_GMAIL_PASSWD", "GMAIL_PASSWD")

	var missing []string
	if from == "" {
		missing = append(missing, "'FROM'")
	}
	if to == "" {
		missing = append(missing, "'TO'")
	}
	if passwd == "" {
		missing = append(missing, "'PASSWD'")
	}
	if len(missing) > 0 {
		logger.Warning("[%s] Gmail provider configuration incomplete: missing required parameter(s) %s", tag, strings.Join(missing, ", "))
		return nil, nil
	}

	if !strings.Contains(from, "@") {
		logger.Warning("[%s] Gmail provider configuration invalid: 'FROM' must be a valid email address", tag)
		return nil, nil
	}

	logLevel := getOption(gmailConf, "LOGLEVEL", "loglevel")
	if logLevel == "" {
		logLevel = "CRITICAL"
	}

	smtpHost := "smtp.gmail.com"
	if host := getOption(gmailConf, "SMTP_HOST", "smtp_host", "HOST", "host", "NOTIF_GMAIL_SMTP_HOST"); host != "" {
		smtpHost = host
	}
	smtpPort := 587
	if portStr := getOption(gmailConf, "SMTP_PORT", "smtp_port", "PORT", "port", "NOTIF_GMAIL_SMTP_PORT"); portStr != "" {
		p, err := strconv.Atoi(portStr)
		if err != nil || p <= 0 || p > 65535 {
			logger.Warning("[%s] Gmail provider configuration invalid: 'SMTP_PORT' must be between 1 and 65535", tag)
			return nil, nil
		}
		smtpPort = p
	}

	logger.Info("[%s] Gmail notification provider initialized", tag)

	return &GmailSender{
		tag:      tag,
		from:     from,
		to:       to,
		smtp:     smtpHost,
		port:     smtpPort,
		passwd:   passwd,
		logLevel: logLevel,
		logger:   logger,
		decrypt:  decrypt,
	}, nil
}

// -----------------------------------------------------------------------------

// SendMessage implements interfaces.INotifSender.
// Parameters:
// - msg: The primary notification body text.
// - attachment: Optional path to a file attachment.
// - source: System or notifier source tag (used to construct the email Subject).
func (gmailSender *GmailSender) SendMessage(ctx context.Context, msg, attachment, source string) error {
	subject := "Notification Alert"
	if source != "" {
		subject = fmt.Sprintf("[%s] Notification", source)
	}

	// Prepare the email payload (subject in header, msg in body)
	emailPayload, err := gmailSender.buildEmail(subject, attachment, msg)
	if err != nil {
		return err
	}

	maxRetries := 3
	backoff := 500 * time.Millisecond
	var lastErr error

	for i := 0; i < maxRetries; i++ {
		// Use a dedicated helper that respects the context
		err = gmailSender.dialAndSend(ctx, emailPayload)
		if err == nil {
			return nil
		}

		lastErr = fmt.Errorf("smtp delivery failed: %v", err)

		if i < maxRetries-1 {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(backoff):
				backoff *= 2
			}
		}
	}

	return fmt.Errorf("gmail send failed after %d retries: %v", maxRetries, lastErr)
}

// -----------------------------------------------------------------------------

// plainAuthWithoutTLSCheck allows PLAIN authentication over connections that are
// already TLS-wrapped at the socket level (such as implicit TLS on Port 465),
// bypassing Go's internal net/smtp Client.tls flag requirement.
type plainAuthWithoutTLSCheck struct {
	identity, username, password, host string
}

func (a *plainAuthWithoutTLSCheck) Start(server *smtp.ServerInfo) (string, []byte, error) {
	if server.Name != a.host {
		return "", nil, fmt.Errorf("wrong host name: got %s, expected %s", server.Name, a.host)
	}
	resp := []byte(a.identity + "\x00" + a.username + "\x00" + a.password)
	return "PLAIN", resp, nil
}

func (a *plainAuthWithoutTLSCheck) Next(fromServer []byte, more bool) ([]byte, error) {
	if more {
		return nil, fmt.Errorf("unexpected server challenge")
	}
	return nil, nil
}

// -----------------------------------------------------------------------------

func (gmailSender *GmailSender) dialAndSend(ctx context.Context, payload []byte) error {
	addr := fmt.Sprintf("%s:%d", gmailSender.smtp, gmailSender.port)
	var conn net.Conn
	var err error

	d := net.Dialer{}

	// 1. Establish connection based on port
	if gmailSender.port == 465 {
		// Implicit TLS
		conn, err = tls.DialWithDialer(&d, "tcp", addr, &tls.Config{ServerName: gmailSender.smtp})
	} else {
		// Plain connection (to be upgraded via STARTTLS)
		conn, err = d.DialContext(ctx, "tcp", addr)
	}

	if err != nil {
		return fmt.Errorf("failed to dial smtp server: %w", err)
	}
	defer conn.Close()

	// 2. Wrap in SMTP client
	c, err := smtp.NewClient(conn, gmailSender.smtp)
	if err != nil {
		return fmt.Errorf("failed to create smtp client: %w", err)
	}
	defer c.Quit()

	// 3. Upgrade to TLS if using 587 (STARTTLS)
	if gmailSender.port != 465 {
		if ok, _ := c.Extension("STARTTLS"); ok {
			config := &tls.Config{ServerName: gmailSender.smtp}
			if err = c.StartTLS(config); err != nil {
				return fmt.Errorf("failed to start tls: %w", err)
			}
		}
	}

	// 4. Authenticate
	// Note: For Gmail with 2-Factor Authentication, PASSWD must be an App Password (16 characters).
	// On Port 465, c.tls is not set by StartTLS, so we use plainAuthWithoutTLSCheck.
	var auth smtp.Auth
	if gmailSender.port == 465 {
		auth = &plainAuthWithoutTLSCheck{"", gmailSender.from, gmailSender.passwd, gmailSender.smtp}
	} else {
		auth = smtp.PlainAuth("", gmailSender.from, gmailSender.passwd, gmailSender.smtp)
	}

	if err = c.Auth(auth); err != nil {
		return fmt.Errorf("authentication failed: %w", err)
	}

	// 5. Set sender and recipient
	if err = c.Mail(gmailSender.from); err != nil {
		return fmt.Errorf("failed to set mail from: %w", err)
	}
	if err = c.Rcpt(gmailSender.to); err != nil {
		return fmt.Errorf("failed to set rcpt to: %w", err)
	}

	// 6. Send data
	w, err := c.Data()
	if err != nil {
		return fmt.Errorf("failed to open data writer: %w", err)
	}
	_, err = w.Write(payload)
	if err != nil {
		return fmt.Errorf("failed to write data: %w", err)
	}
	err = w.Close()
	if err != nil {
		return fmt.Errorf("failed to close data writer: %w", err)
	}

	return nil
}

// -----------------------------------------------------------------------------

func (gmailSender *GmailSender) buildEmail(subject, attachment, body string) ([]byte, error) {
	var email bytes.Buffer
	writer := multipart.NewWriter(&email)

	// Headers
	email.WriteString(fmt.Sprintf("From: %s\r\n", gmailSender.from))
	email.WriteString(fmt.Sprintf("To: %s\r\n", gmailSender.to))
	email.WriteString(fmt.Sprintf("Subject: %s\r\n", subject))
	email.WriteString("MIME-Version: 1.0\r\n")
	email.WriteString(fmt.Sprintf("Content-Type: multipart/mixed; boundary=%s\r\n", writer.Boundary()))
	email.WriteString("\r\n")

	// Body
	bodyPart, err := writer.CreatePart(map[string][]string{"Content-Type": {"text/plain; charset=\"utf-8\""}})
	if err != nil {
		return nil, err
	}
	bodyPart.Write([]byte(body))

	// Attachment
	if attachment != "" {
		attachmentBytes, err := os.ReadFile(attachment)
		if err != nil {
			return nil, fmt.Errorf("failed to read attachment: %w", err)
		}

		h := make(map[string][]string)
		h["Content-Type"] = []string{fmt.Sprintf("application/octet-stream; name=\"%s\"", filepath.Base(attachment))}
		h["Content-Transfer-Encoding"] = []string{"base64"}
		h["Content-Disposition"] = []string{fmt.Sprintf("attachment; filename=\"%s\"", filepath.Base(attachment))}

		part, err := writer.CreatePart(h)
		if err != nil {
			return nil, err
		}

		encoded := make([]byte, base64.StdEncoding.EncodedLen(len(attachmentBytes)))
		base64.StdEncoding.Encode(encoded, attachmentBytes)
		part.Write(encoded)
	}

	writer.Close()
	return email.Bytes(), nil
}

// -----------------------------------------------------------------------------

func (g *GmailSender) GetTag() string {
	return g.tag
}

// -----------------------------------------------------------------------------

func (g *GmailSender) GetLogLevel() string {
	return g.logLevel
}
