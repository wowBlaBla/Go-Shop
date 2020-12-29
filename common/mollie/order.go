package mollie

import "time"

type Order struct {
	Id string `json:"id,omitempty"`
	CreatedAt *time.Time `json:"createdAt,omitempty"`
	ExpiresAt *time.Time `json:"expiresAt,omitempty"`
	//
	Lines []Line `json:"lines"`
	Amount Amount `json:"amount"`
	BillingAddress Address `json:"billingAddress,omitempty"`
	ShippingAddress Address `json:"shippingAddress,omitempty"`
	Metadata map[string]string `json:"metadata,omitempty"`
	ConsumerDateOfBirth string `json:"consumerDateOfBirth,omitempty"` // '1958-01-31'
	Locale string `json:"locale,omitempty"`
	OrderNumber string `json:"orderNumber,omitempty"`
	RedirectUrl string `json:"redirectUrl,omitempty"`
	WebhookUrl string `json:"webhookUrl,omitempty"`
	Method string `json:"method,omitempty"`
	//
	Status string `json:"status,omitempty"`
	IsCancelable bool `json:"isCancelable,omitempty"`
	ProfileId string `json:"profileId,omitempty"`
}