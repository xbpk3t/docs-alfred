// Package mail provides thin helpers for sending HTML email via Resend.
package mail

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/resend/resend-go/v2"
)

// SendOptions is a single HTML email send (no config/env loading).
type SendOptions struct {
	Token   string
	From    string
	Subject string
	HTML    string
	To      []string
}

// DefaultFrom builds `name <onboarding@resend.dev>` (name default "noreply" if empty).
func DefaultFrom(name string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		name = "noreply"
	}
	return name + " <onboarding@resend.dev>"
}

// ParseAddresses splits comma-separated emails; trims space; drops empties.
func ParseAddresses(s string) []string {
	if strings.TrimSpace(s) == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	var out []string
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

// SendHTML sends one HTML email via Resend.
func SendHTML(ctx context.Context, opts *SendOptions) error {
	if opts == nil {
		return fmt.Errorf("mail options is required")
	}
	if opts.Token == "" {
		return fmt.Errorf("mail token is required")
	}
	if len(opts.To) == 0 {
		return fmt.Errorf("mail To is required")
	}
	if strings.TrimSpace(opts.From) == "" {
		return fmt.Errorf("mail From is required")
	}
	if strings.TrimSpace(opts.Subject) == "" {
		return fmt.Errorf("mail Subject is required")
	}

	client := resend.NewClient(opts.Token)
	params := &resend.SendEmailRequest{
		From:    opts.From,
		To:      opts.To,
		Subject: opts.Subject,
		Html:    opts.HTML,
	}

	slog.Info("sending email", "to", opts.To, "subject", opts.Subject, "from", opts.From)
	sent, err := client.Emails.SendWithContext(ctx, params)
	if err != nil {
		return fmt.Errorf("send email: %w", err)
	}
	slog.Info("email sent", "id", sent.Id)
	return nil
}
