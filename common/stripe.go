package common

import (
	"fmt"
	"github.com/stripe/stripe-go/v71"
	"github.com/stripe/stripe-go/v71/balance"
	"github.com/stripe/stripe-go/v71/card"
	"github.com/stripe/stripe-go/v71/customer"
	"github.com/stripe/stripe-go/v71/paymentintent"
)

func NewStripe(key string) *Stripe {
	return &Stripe{Key: key}
}

type Stripe struct {
	Key string
}

func (s *Stripe) GetBalance() (*stripe.Balance, error) {
	stripe.Key = s.Key
	b, err := balance.Get(nil)
	if err != nil {
		return nil, err
	}
	return b, nil
}

/* Customers */
func (s *Stripe) GetCustomers() ([]*stripe.Customer, error) {
	stripe.Key = s.Key
	var customers []*stripe.Customer
	params := &stripe.CustomerListParams{}
	i := customer.List(params)
	for i.Next() {
		customers = append(customers, i.Customer())
	}
	return customers, nil
}

func (s *Stripe) CreateCustomer(params *stripe.CustomerParams) (*stripe.Customer, error) {
	stripe.Key = s.Key
	c, err := customer.New(params)
	if err != nil {
		return nil, err
	}
	return c, nil
}

func (s *Stripe) GetCustomer(id string) (*stripe.Customer, error) {
	stripe.Key = s.Key
	c, err := customer.Get(
		id,nil,
	)
	if err != nil {
		return nil, err
	}
	if c.Deleted {
		return nil, fmt.Errorf("deleted")
	}
	return c, nil
}

/* PaymentIntent */
func (s *Stripe) GetPaymentIntents() ([]*stripe.PaymentIntent, error) {
	stripe.Key = s.Key
	var paymentIntents []*stripe.PaymentIntent
	i := paymentintent.List(&stripe.PaymentIntentListParams{})
	for i.Next() {
		paymentIntents = append(paymentIntents, i.PaymentIntent())
	}
	return paymentIntents, nil
}

func (s *Stripe) CreatePaymentIntent(params *stripe.PaymentIntentParams) (*stripe.PaymentIntent, error) {
	stripe.Key = s.Key
	c, err := paymentintent.New(params)
	if err != nil {
		return nil, err
	}
	return c, nil
}

func (s *Stripe) GetPaymentIntent(id string) (*stripe.PaymentIntent, error) {
	stripe.Key = s.Key
	c, err := paymentintent.Get(
		id,nil,
	)
	if err != nil {
		return nil, err
	}
	return c, nil
}

/* Card */
func (s *Stripe) GetCards(customerId string) ([]*stripe.Card, error) {
	stripe.Key = s.Key
	var cards []*stripe.Card
	params := &stripe.CardListParams{
		Customer: stripe.String(customerId),
	}
	i := card.List(params)
	for i.Next() {
		cards = append(cards, i.Card())
	}
	return cards, nil
}

func (s *Stripe) CreateCard(params *stripe.CardParams) (*stripe.Card, error) {
	stripe.Key = s.Key
	c, err := card.New(params)
	if err != nil {
		return nil, err
	}
	return c, nil
}

func (s *Stripe) GetCard(customerId, id string) (*stripe.Card, error) {
	stripe.Key = s.Key
	t, err := card.Get(
		id, &stripe.CardParams{
			Customer: stripe.String(customerId),
		},
	)
	if err != nil {
		return nil, err
	}
	return t, nil
}

func (s *Stripe) DeleteCard(customerId, id string) (*stripe.Card, error) {
	stripe.Key = s.Key
	t, err := card.Del(
		id, &stripe.CardParams{
			Customer: stripe.String(customerId),
		},
	)
	if err != nil {
		return nil, err
	}
	return t, nil
}