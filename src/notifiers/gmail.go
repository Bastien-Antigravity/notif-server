package notifiers

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"mime/multipart"
	"net/smtp"
	"os"
	"path/filepath"
)

type GmailSender struct {
	tag string
	// used for email
	from string // one sender-> one receiver
	to   string // one sender-> one receiver
	// config
	smtp   string
	port   int32
	passwd string
	// Level
	logLevel string
}

func NewGmailSender(gmailConf map[string]string, confName string) (*GmailSender, string) {
	curError := ""
	gmailSender := &GmailSender{}
	if tag, ok := gmailConf["TAG"]; ok {
		gmailSender.tag = tag
	} else {
		curError += fmt.Sprintf("missing 'TAG' option for config '%s'\n", confName)
	}
	if gmailFrom, ok := gmailConf["FROM"]; ok {
		gmailSender.from = gmailFrom
	} else {
		curError += fmt.Sprintf("missing 'FROM' option for config '%s'\n", confName)
	}
	if gmailTo, ok := gmailConf["TO"]; ok {
		gmailSender.to = gmailTo
	} else {
		curError += fmt.Sprintf("missing 'TO' option for config '%s'\n", confName)
	}
	if gmailPasswd, ok := gmailConf["PASSWD"]; ok {
		gmailSender.passwd = gmailPasswd
	} else {
		curError += fmt.Sprintf("missing 'PASSWD' option for config '%s'\n", confName)
	}
	if logLevel, ok := gmailConf["LOGLEVEL"]; ok {
		gmailSender.logLevel = logLevel
	} else {
		curError += fmt.Sprintf("missing 'LOGLEVEL' option for config '%s'\n", confName)
	}
	if curError == "" {
		gmailSender.smtp = "smtp.gmail.com"
		gmailSender.port = 587
		return gmailSender, ""
	}
	return nil, curError
}

func (gmailSender *GmailSender) SendMessage(subject, attachment, body string) error {
	// prepare attachment
	var attachmentBytes []byte
	var err error
	var attachmentName string
	var encodedAttachment string

	if attachment != "" {
		attachmentBytes, err = os.ReadFile(attachment)
		if err != nil {
			return fmt.Errorf("failed to read attachment (gmail): %v", err)
		}
		attachmentName = filepath.Base(attachment)
		encodedAttachment = base64.StdEncoding.EncodeToString(attachmentBytes)
	}

	// create email buffer
	var email bytes.Buffer
	writer := multipart.NewWriter(&email)
	// Add email headers
	email.WriteString(fmt.Sprintf("From: %s\r\n", gmailSender.from))
	email.WriteString(fmt.Sprintf("To: %s\r\n", gmailSender.to))
	email.WriteString(fmt.Sprintf("Subject: %s\r\n", subject))
	email.WriteString("MIME-Version: 1.0\r\n")
	email.WriteString(fmt.Sprintf("Content-Type: multipart/mixed; boundary=%s\r\n", writer.Boundary()))
	email.WriteString("\r\n")

	// Add email body
	bodyPart, err := writer.CreatePart(
		map[string][]string{
			"Content-Type": {"text/plain; charset=\"utf-8\""},
		},
	)
	if err != nil {
		return fmt.Errorf("failed to create body part (gmail): %v", err)
	}
	bodyPart.Write([]byte(body))

	// Add attachment if exists
	if attachment != "" {
		attachmentPart, err := writer.CreatePart(
			map[string][]string{
				"Content-Type":              {fmt.Sprintf("application/octet-stream; name=\"%s\"", attachmentName)},
				"Content-Transfer-Encoding": {"base64"},
				"Content-Disposition":       {fmt.Sprintf("attachment; filename=\"%s\"", attachmentName)},
			},
		)
		if err != nil {
			return fmt.Errorf("failed to create attachment part (gmail): %v", err)
		}
		attachmentPart.Write([]byte(encodedAttachment))
	}

	// Close multipart writer
	writer.Close()
	// Create authentication
	auth := smtp.PlainAuth("", gmailSender.from, gmailSender.passwd, gmailSender.smtp)
	// Send email
	err = smtp.SendMail(
		fmt.Sprintf("%s:%d", gmailSender.smtp, gmailSender.port),
		auth,
		gmailSender.from,
		[]string{gmailSender.to},
		email.Bytes(),
	)
	if err != nil {
		return fmt.Errorf("error while trying to send email (gmail): %v", err)
	}
	return nil
}

func (g *GmailSender) GetTag() string {
	return g.tag
}

func (g *GmailSender) GetLogLevel() string {
	return g.logLevel
}
