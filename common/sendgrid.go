package common

import (
	"fmt"
	"github.com/google/logger"
	"github.com/sendgrid/sendgrid-go"
	"github.com/sendgrid/sendgrid-go/helpers/mail"
	"net/http"
)

func NewSendGrid(key string) *SendGrid {
	return &SendGrid{Key: key}
}

type SendGrid struct {
	Key string
}

func (s *SendGrid) Send(from, to *mail.Email, title, plain, html string) error {
	message := mail.NewSingleEmail(from, title, to, plain, html)
	client := sendgrid.NewSendClient(s.Key)
	if resp, err := client.Send(message); err == nil {
		if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusAccepted {
			logger.Errorf("%+v", resp.Body)
			return fmt.Errorf("status %v", resp.StatusCode)
		}
	} else {
		return err
	}
	return nil
}