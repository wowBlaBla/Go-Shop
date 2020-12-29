package mollie

import (
	"fmt"
	"time"
)

// https://docs.mollie.com/reference/v2/payments-api/create-payment
type Payment struct {
	Id string `json:"id,omitempty"`
	CreatedAt time.Time `json:"createdAt,omitempty"`
	Status string `json:"status,omitempty"`
	IsCancelable bool `json:"isCancelable,omitempty"`
	ExpiresAt time.Time `json:"expiresAt,omitempty"`
	ProfileId string `json:"profileId,omitempty"`
	//
	Amount Amount `json:"amount"`
	Description string `json:"description"`
	RedirectUrl string `json:"redirectUrl"`
	//
	WebhookUrl string `json:"webhookUrl,omitempty"`
	Locale string `json:"locale,omitempty"`
	// Method string `json:"method,omitempty"` Ex.: ['bancontact', 'belfius', 'inghomepay']
	Metadata map[string]string `json:"metadata,omitempty"`
	SequenceType string `json:"sequenceType,omitempty"` // 'oneoff' 'first' 'recurring'
	CustomerId string `json:"customerId,omitempty"`
	MandateId string `json:"mandateId,omitempty"`
	RestrictPaymentMethodsToCountry string `json:"restrictPaymentMethodsToCountry,omitempty"`
}

func NewAmount(currency string, value float64) Amount {
	return Amount{Currency: currency, Value: fmt.Sprintf("%.2f", value)}
}

type Amount struct {
	Currency string `json:"currency,omitempty"` // USD, EUR
	Value string `json:"value,omitempty"`
}