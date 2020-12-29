package common

import (
	"github.com/yonnic/goshop/common/mollie"
	"log"
	"testing"
)

func TestMollie_GetOrders1(t *testing.T) {
	m := NewMollie("test_wRWyp7f3QxCETByjcHfBrbMu3RtEyM")
	if orders, err := m.GetOrders(); err == nil {
		log.Printf("Orders: %+v", len(orders))
	}else{
		t.Fatalf("%+v", err)
	}
}

func TestMollie_CreateOrder(t *testing.T) {
	m := NewMollie("test_wRWyp7f3QxCETByjcHfBrbMu3RtEyM")
	address := mollie.Address{
		Title:            "Mr",
		GivenName:        "John",
		FamilyName:       "Smith",
		Email:            "john@smith.com",
		Phone:            "+49123456789",
		OrganizationName: "acme",
		StreetAndNumber:  "First 1/23",
		City:             "Berlin",
		Region:           "Berlin",
		PostalCode:       "123456",
		Country:          "DE",
	}
	if order, _, err := m.CreateOrder(&mollie.Order{
		Amount:              mollie.NewAmount("USD", 123.45),
		BillingAddress:      address,
		ShippingAddress:     address,
		Lines: []mollie.Line{{
				Type:           "physical",
				Category:       "gift",
				Sku:            "1.2.3.4.5",
				Name:           "Test 1",
				Metadata:       map[string]string{"variation": "1"},
				Quantity:       1,
				VatRate: 		"21.0",
				UnitPrice:      mollie.NewAmount("USD", 123.45),
				TotalAmount:    mollie.NewAmount("USD", 123.45),
				DiscountAmount: mollie.NewAmount("USD", 0),
				VatAmount:      mollie.NewAmount("USD", 21.43),
		}},
		Metadata:            map[string]string{"extra": "value"},
		OrderNumber:         "1",
		RedirectUrl:         "https://shop.servhost.org/api/v1/account/orders/1/mollie/success",
		//Method:              "",
		Locale: "de_DE",
	}); err == nil {
		log.Printf("order: %+v", order)
	}else{
		t.Fatalf("%+v", err)
	}
}

func TestMollie_GetOrder(t *testing.T) {
	m := NewMollie("test_wRWyp7f3QxCETByjcHfBrbMu3RtEyM")
	if order, err := m.GetOrder("ord_zo63y"); err == nil {
		log.Printf("order: %+v", order)
	}else{
		t.Fatalf("%+v", err)
	}
}

func TestMollie_DeleteOrder(t *testing.T) {
	m := NewMollie("test_wRWyp7f3QxCETByjcHfBrbMu3RtEyM")
	if order, err := m.DeleteOrder("ord_zo63y"); err == nil {
		log.Printf("order: %+v", order)
	}else{
		t.Fatalf("%+v", err)
	}
}

//

func TestMollie_GetPayments1(t *testing.T) {
	m := NewMollie("test_wRWyp7f3QxCETByjcHfBrbMu3RtEyM")
	if payments, err := m.GetPayments(); err == nil {
		log.Printf("Payments: %+v", len(payments))
	}else{
		t.Fatalf("%+v", err)
	}
}

func TestMollie_CreatePayment(t *testing.T) {
	m := NewMollie("test_wRWyp7f3QxCETByjcHfBrbMu3RtEyM")
	if payment, err := m.CreatePayment(&mollie.Payment{Amount: mollie.NewAmount("USD", 123.45), Description: "order: 1", RedirectUrl: "http://shop.servhost.org/api/v1/account/orders/1/stripe/success"}); err == nil {
		log.Printf("payment: %+v", payment)
	}else{
		t.Fatalf("%+v", err)
	}
	// tr_65Fy8nt6fU
}

func TestMollie_GetPayment(t *testing.T) {
	m := NewMollie("test_wRWyp7f3QxCETByjcHfBrbMu3RtEyM")
	if payment, err := m.GetPayment("tr_65Fy8nt6fU"); err == nil {
		log.Printf("payment: %+v", payment)
	}else{
		t.Fatalf("%+v", err)
	}
	//
}