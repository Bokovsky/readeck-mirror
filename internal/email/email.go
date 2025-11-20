// SPDX-FileCopyrightText: © 2021 Olivier Meunier <olivier@neokraft.net>
//
// SPDX-License-Identifier: AGPL-3.0-only

// Package email provides functions to send emails.
package email

import (
	"bytes"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/CloudyKit/jet/v6"
	"github.com/aymerick/douceur/inliner"
	"github.com/wneessen/go-mail"

	"codeberg.org/readeck/readeck/assets"
	"codeberg.org/readeck/readeck/configs"
	"codeberg.org/readeck/readeck/internal/templates"
)

// Sender is the default email sender. It's made public so it can be
// overridden during tests.
var Sender sender

// views hold all the templates.
var views *jet.Set

// InitSender initializes the default email sender base on the
// configuration.
func InitSender() {
	views = templates.Catalog(
		os.DirFS(configs.Config.Customize.ExtraTemplates),
	)

	if Sender != nil { // Sender can be set by the test runner
		return
	}

	if configs.Config.Email.Debug {
		Sender = &StdOutSender{}
	} else if configs.Config.Email.Host != "" {
		Sender = &SMTPSender{}
	}

	if Sender == nil {
		return
	}
}

// CanSendEmail returns true when we can send emails.
func CanSendEmail() bool {
	return Sender != nil
}

// MessageOption is a function that can manipulate a message and is called
// during [NewMsg].
type MessageOption func(msg *mail.Msg) error

// NewMsg creates a new [mail.Msg] with sender, recipient and subject.
// It checks if we can send email and adds a User-Agent header.
func NewMsg(from, to, subject string, options ...MessageOption) (*mail.Msg, error) {
	if !CanSendEmail() {
		return nil, errors.New("no email sender defined")
	}

	msg := mail.NewMsg()
	if err := msg.From(from); err != nil {
		return nil, err
	}
	if err := msg.AddTo(to); err != nil {
		return nil, err
	}
	msg.Subject(subject)
	msg.SetUserAgent("Readeck // https://readeck.org/")

	for _, fn := range options {
		if err := fn(msg); err != nil {
			return nil, err
		}
	}

	return msg, nil
}

// WithMDTemplate is a [MessageOption] that adds a text/plain message part using
// a markdown template.
// The text is then converted to HTML and passed to [WithHTMLTemplate] so the message
// gets an automatic HTML part as well.
func WithMDTemplate(template string, vars jet.VarMap, data map[string]any) MessageOption {
	return func(msg *mail.Msg) error {
		tpl, err := views.GetTemplate(template)
		if err != nil {
			return err
		}

		// Render text part
		txt := new(bytes.Buffer)
		if err = tpl.Execute(txt, vars, data); err != nil {
			return err
		}

		// Convert to HTML
		html := new(bytes.Buffer)
		if err = markdown.Convert(txt.Bytes(), html); err != nil {
			return err
		}

		// Add footer to text part
		if tpl, err = views.GetTemplate("/emails/include/footer.jet.md"); err != nil {
			return err
		}
		if err = tpl.Execute(txt, vars, data); err != nil {
			return err
		}

		// Add text part
		msg.AddAlternativeString(mail.TypeTextPlain, txt.String())

		// Add HTML part
		data["HTML"] = html
		return WithHTMLTemplate("/emails/include/markdown", vars, data)(msg)
	}
}

// WithHTMLTemplate is a [MessageOption] that adds a text/html message part using a
// given template.
// A stylesheet is made available to the template and is later inlined in the
// message for greater compatibility with email clients.
func WithHTMLTemplate(template string, vars jet.VarMap, data map[string]any) MessageOption {
	return func(msg *mail.Msg) error {
		// Load the email CSS file.
		var fd io.ReadCloser
		var err error
		fd, err = assets.StaticFilesFS().Open("email.css")
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				// If the file does not exist, just ignore it.
				fd = io.NopCloser(new(bytes.Buffer))
			} else {
				return err
			}
		}
		css, err := io.ReadAll(fd)
		if err != nil {
			fd.Close() // nolint:errcheck
			return err
		}
		fd.Close() // nolint:errcheck

		// Render the given template.
		data["CSS"] = string(css)

		w := new(bytes.Buffer)
		tpl, err := views.GetTemplate(template)
		if err != nil {
			return err
		}
		if err = tpl.Execute(w, vars, data); err != nil {
			return err
		}

		// Create a text/plain version when there isn't one already.
		// text/plain needs to be the first part.
		hasTextPlain := false
		for _, p := range msg.GetParts() {
			if p.GetContentType() == mail.TypeTextPlain {
				hasTextPlain = true
				break
			}
		}
		if !hasTextPlain {
			txt, err := html2md4email.ConvertString(w.String())
			if err != nil {
				return err
			}
			msg.AddAlternativeString(mail.TypeTextPlain, txt)
		}

		// Inline CSS in HTML tags.
		html, err := inliner.Inline(w.String())
		if err != nil {
			return err
		}

		// Set HTML body
		msg.AddAlternativeString(mail.TypeTextHTML, html)
		return nil
	}
}

// sender defines an email sender.
type sender interface {
	SendEmail(*mail.Msg) error
}

// StdOutSender implements EmailSender for stdout.
type StdOutSender struct{}

// SendEmail "sends" an email to stdout.
func (s *StdOutSender) SendEmail(msg *mail.Msg) error {
	fmt.Fprintln(os.Stdout, "=== Outbound email ===================================================")
	msg.WriteTo(os.Stdout) // nolint:errcheck
	fmt.Fprintln(os.Stdout, "\n======================================================================")
	return nil
}

// SMTPSender implements EmailSender for SMTP.
type SMTPSender struct{}

// SendEmail sends an email using SMTP.
func (s *SMTPSender) SendEmail(msg *mail.Msg) error {
	client, err := mail.NewClient(
		configs.Config.Email.Host,
		mail.WithPort(configs.Config.Email.Port),
		mail.WithTimeout(time.Second*10),
		mail.WithTLSConfig(&tls.Config{
			ServerName:         configs.Config.Email.Host,
			MinVersion:         mail.DefaultTLSMinVersion,
			InsecureSkipVerify: configs.Config.Email.Insecure, //nolint:gosec
		}),
	)
	if err != nil {
		return err
	}

	if configs.Config.Email.Username != "" {
		client.SetSMTPAuth(mail.SMTPAuthPlain)
		client.SetUsername(configs.Config.Email.Username)
		client.SetPassword(configs.Config.Email.Password)
	}

	switch configs.Config.Email.Encryption {
	case "starttls":
		client.SetTLSPolicy(mail.TLSMandatory)
	case "ssltls":
		client.SetSSL(true)
	case "none":
		client.SetTLSPolicy(mail.NoTLS)
	default:
		client.SetTLSPolicy(mail.TLSOpportunistic)
	}

	return client.DialAndSend(msg)
}
