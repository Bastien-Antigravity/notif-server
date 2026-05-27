package notifiers

/*
ESSENTIAL PROCESS:
Implements the Gmail notification sender via SMTP with architectural hardening.
Handles both Port 587 (STARTTLS) and Port 465 (Implicit TLS).

DATA FLOW:
1. Receives message payload, subject, and optional attachment path.
2. Dial the SMTP server using net.Dialer with context support.
3. Upgrades to TLS (if 587) or starts with TLS (if 465).
4. Executes SMTP state machine (Auth -> Mail -> Rcpt -> Data).
5. Respects context deadlines throughout the network operation.

KEY PARAMETERS:
- from: Sender email address.
- to: Recipient email address.
- smtp: SMTP server address (default: smtp.gmail.com).
- port: SMTP server port (default: 587).
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
	"time"
)

type GmailSender struct {
	tag      string
	from     string
	to       string
	smtp     string
	port     int
	passwd   string
	logLevel string
}

// -----------------------------------------------------------------------------

func NewGmailSender(gmailConf map[string]string, confName string) (*GmailSender, string) {
	curError := ""
	gmailSender := &GmailSender{
		smtp: "smtp.gmail.com",
		port: 587,
	}

	if tag, ok := gmailConf["TAG"]; ok {
		gmailSender.tag = tag
	} else {
		curError += fmt.Sprintf("missing 'TAG' option for config '%s'\n", confName)
	}

	if from, ok := gmailConf["FROM"]; ok {
		gmailSender.from = from
	} else {
		curError += fmt.Sprintf("missing 'FROM' option for config '%s'\n", confName)
	}

	if to, ok := gmailConf["TO"]; ok {
		gmailSender.to = to
	} else {
		curError += fmt.Sprintf("missing 'TO' option for config '%s'\n", confName)
	}

	if passwd, ok := gmailConf["PASSWD"]; ok {
		gmailSender.passwd = passwd
	} else {
		curError += fmt.Sprintf("missing 'PASSWD' option for config '%s'\n", confName)
	}

	if logLevel, ok := gmailConf["LOGLEVEL"]; ok {
		gmailSender.logLevel = logLevel
	} else {
		curError += fmt.Sprintf("missing 'LOGLEVEL' option for config '%s'\n", confName)
	}

	// Optional overrides
	if host, ok := gmailConf["SMTP_HOST"]; ok {
		gmailSender.smtp = host
	}
	if portStr, ok := gmailConf["SMTP_PORT"]; ok {
		if p, err := strconv.Atoi(portStr); err == nil {
			gmailSender.port = p
		}
	}

	if curError == "" {
		return gmailSender, ""
	}
	return nil, curError
}

// -----------------------------------------------------------------------------

func (gmailSender *GmailSender) SendMessage(ctx context.Context, subject, attachment, body string) error {
	// Prepare the email payload
	emailPayload, err := gmailSender.buildEmail(subject, attachment, body)
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
	auth := smtp.PlainAuth("", gmailSender.from, gmailSender.passwd, gmailSender.smtp)
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
