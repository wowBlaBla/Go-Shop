package common

import (
	"bytes"
	"github.com/sendgrid/sendgrid-go/helpers/mail"
	"html/template"
	"regexp"
	"strings"
)

const (
	NOTIFICATION_TYPE_ADMIN_ORDER_PAID = "admin-order-paid"
	NOTIFICATION_TYPE_USER_ORDER_PAID = "user-order-paid"
)

var (
	funcMap = template.FuncMap{
		"add": add,
		"even": even,
		"odd": odd,
		"toUuid":  toUuid,
	}
)

func add(a, b int) int {
	return a + b
}

func even(i int) bool {
	return i % 2 == 0
}

func odd(i int) bool {
	return !even(i)
}

func toUuid(raw string) string {
	re := regexp.MustCompile(`^\[(.*)\]$`)
	return strings.Replace(re.ReplaceAllString(raw, "$1"), ",", ".", -1)
}

func NewNotification() *Notification{
	return &Notification{}
}

type Notification struct {
	SendGrid *SendGrid
}

type NotificationTemplateVariables struct {
	Order interface{}
}

func (n *Notification) SendEmail(from, to *mail.Email, topic, message string, vars *NotificationTemplateVariables) error {
	if tmpl, err := template.New("topic").Funcs(funcMap).Parse(topic); err == nil {
		var tpl bytes.Buffer
		if err := tmpl.Execute(&tpl, vars); err == nil {
			topic = tpl.String()
		}else{
			return err
		}
	}else{
		return err
	}
	if tmpl, err := template.New("message").Funcs(funcMap).Parse(message); err == nil {
		var tpl bytes.Buffer
		if err := tmpl.Execute(&tpl, vars); err == nil {
			message = tpl.String()
		}else{
			return err
		}
	}else{
		return err
	}
	//
	//logger.Infof("From: %+v, To: %+v, Topic: %+v, Message: %+v", from, to, topic, message)
	//
	return n.SendGrid.Send(from, to, topic, message, message)
}