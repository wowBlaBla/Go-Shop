package common

import (
	"bytes"
	"encoding/json"
	"github.com/google/logger"
	"github.com/yonnic/goshop/common/mollie"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const MOLLIE_API = "https://api.mollie.com/v2"

/*
Test API key: test_t42DxbEjtNCq7P4SD3KEUMCmSQw5rD
Partner ID: 11120627
Profile ID: pfl_qcRqsdFUKm
*/

func NewMollie(key string) *Mollie {
	return &Mollie{Key: key}
}

type Mollie struct {
	Key string
}

func (m Mollie) getClient() *http.Client {
	return &http.Client{
		Timeout: time.Duration(15) * time.Second,
	}
}

func (m Mollie) newRequest(method, url string, body io.Reader) (*http.Request, error) {
	req, err := http.NewRequest(method, url, body)
	if err == nil {
		req.Header.Add("Authorization", "Bearer " + m.Key)
	}
	return req, err
}

// Orders
func (m Mollie) GetOrders() ([]mollie.Order, error) {
	var orders []mollie.Order
	client := m.getClient()
	req, _ := m.newRequest(http.MethodGet, MOLLIE_API + "/orders", nil)
	if resp, err := client.Do(req); err == nil {
		defer resp.Body.Close()
		bts, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			return orders, err
		}
		var response struct {
			Count int `json:"count"`
			Embedded struct {
				Orders []mollie.Order `json:"orders,omitempty"`
			} `json:"_embedded,omitempty"`
			Links struct {
				Self *mollie.Link `json:"self,omitempty"`
				Previous *mollie.Link `json:"previous,omitempty"`
				Next *mollie.Link `json:"next,omitempty"`
				Documentation *mollie.Link `json:"documentation,omitempty"`
			} `json:"_links,omitempty"`
		}
		if err = json.Unmarshal(bts, &response); err != nil {
			return orders, err
		}
		return orders, nil
	}else{
		return orders, err
	}
}

func (m Mollie) CreateOrder(params *mollie.Order) (*mollie.Order, map[string]*mollie.Link, error) {
	client := m.getClient()
	bts, err := json.Marshal(params)
	if err != nil {
		return nil, nil, err
	}
	req, _ := m.newRequest(http.MethodPost, MOLLIE_API + "/orders", bytes.NewBuffer(bts))
	log.Printf("req: %+v", req)
	if resp, err := client.Do(req); err == nil {
		defer resp.Body.Close()
		bts, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			return nil, nil, err
		}
		logger.Infof("Bts: %+v", string(bts))
		var response struct {
			*mollie.Order
			Links map[string]*mollie.Link `json:"_links,omitempty"` // dashboard, documentation, checkout, self
		}
		if err = json.Unmarshal(bts, &response); err != nil {
			return nil, nil, err
		}
		log.Printf("Response: %+v", response)
		return response.Order, response.Links, nil
	}
	return nil, nil, nil
}

func (m Mollie) GetOrder(id string) (*mollie.Order, error) {
	client := m.getClient()
	req, _ := m.newRequest(http.MethodGet, MOLLIE_API + "/orders/" + id + "?embed=payments", nil)
	if resp, err := client.Do(req); err == nil {
		defer resp.Body.Close()
		bts, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			return nil, err
		}
		var response struct {
			*mollie.Order
			Links struct {
				Self *mollie.Link `json:"self,omitempty"`
				Checkout *mollie.Link `json:"checkout,omitempty"`
				Dashboard *mollie.Link `json:"dashboard,omitempty"`
				Documentation *mollie.Link `json:"documentation,omitempty"`
			} `json:"_links,omitempty"`
			Embedded struct {
				Payments []mollie.Payment `json:"payments,omitempty"`
			} `json:"_embedded,omitempty"`
		}
		if err = json.Unmarshal(bts, &response); err != nil {
			return nil, err
		}
		return response.Order, nil
	}else{
		return nil, err
	}
}

func (m Mollie) DeleteOrder(id string) (*mollie.Order, error) {
	client := m.getClient()
	req, _ := m.newRequest(http.MethodDelete, MOLLIE_API + "/orders/" + id, nil)
	if resp, err := client.Do(req); err == nil {
		defer resp.Body.Close()
		bts, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			return nil, err
		}
		var response struct {
			*mollie.Order
			Links struct {
				Self *mollie.Link `json:"self,omitempty"`
				Checkout *mollie.Link `json:"checkout,omitempty"`
				Dashboard *mollie.Link `json:"dashboard,omitempty"`
				Documentation *mollie.Link `json:"documentation,omitempty"`
			} `json:"_links,omitempty"`
		}
		if err = json.Unmarshal(bts, &response); err != nil {
			return nil, err
		}
		log.Printf("Response: %+v", response)
		return response.Order, nil
	}
	return nil, nil
}

// Payments
func (m Mollie) GetPayments() ([]mollie.Payment, error) {
	var payments []mollie.Payment
	client := m.getClient()
	req, _ := m.newRequest(http.MethodGet, MOLLIE_API + "/payments", nil)
	if resp, err := client.Do(req); err == nil {
		defer resp.Body.Close()
		bts, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			return payments, err
		}
		var response struct {
			Count int `json:"count"`
			Embedded struct {
				Payments []mollie.Payment `json:"payments,omitempty"`
			} `json:"_embedded,omitempty"`
			Links struct {
				Self *mollie.Link `json:"self,omitempty"`
				Previous *mollie.Link `json:"previous,omitempty"`
				Next *mollie.Link `json:"next,omitempty"`
				Documentation *mollie.Link `json:"documentation,omitempty"`
			} `json:"_links,omitempty"`
		}
		if err = json.Unmarshal(bts, &response); err != nil {
			return payments, err
		}
		return payments, nil
	}else{
		return payments, err
	}
}

func (m Mollie) CreatePayment(params *mollie.Payment) (*mollie.Payment, error) {
	var payment *mollie.Payment
	client := m.getClient()
	data := url.Values{}
	data.Set("amount[currency]", params.Amount.Currency)
	data.Set("amount[value]", params.Amount.Value)
	data.Set("description", params.Description)
	data.Set("redirectUrl", params.RedirectUrl)
	if len(params.Metadata) > 0 {
		if bts, err := json.Marshal(params.Metadata); err == nil {
			data.Set("metadata", string(bts))
		}
	}
	req, _ := m.newRequest(http.MethodPost, MOLLIE_API + "/payments", strings.NewReader(data.Encode()))
	req.Header.Add("Content-Type", "application/x-www-form-urlencoded")
	if resp, err := client.Do(req); err == nil {
		defer resp.Body.Close()
		bts, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			return payment, err
		}
		log.Printf("Bts: %+v", string(bts))
		var response struct {
			Id string `json:"id"`
			CreatedAt time.Time `json:"createdAt"`
			Status string `json:"status"`
			IsCancelable bool `json:"isCancelable"`
			ExpiresAt time.Time `json:"expiresAt"`
			ProfileId string `json:"profileId"`
			Links struct {
				Self *mollie.Link `json:"self,omitempty"`
				Checkout *mollie.Link `json:"checkout,omitempty"`
				Dashboard *mollie.Link `json:"dashboard,omitempty"`
				Documentation *mollie.Link `json:"documentation,omitempty"`
			} `json:"_links,omitempty"`
		}
		if err = json.Unmarshal(bts, &response); err != nil {
			return payment, err
		}
		log.Printf("Response: %+v", response)
		return payment, nil
	}
	return payment, nil
}

func (m Mollie) GetPayment(id string) (*mollie.Payment, error) {
	client := m.getClient()
	req, _ := m.newRequest(http.MethodGet, MOLLIE_API + "/payments/" + id, nil)
	if resp, err := client.Do(req); err == nil {
		defer resp.Body.Close()
		bts, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			return nil, err
		}
		log.Printf("Bts: %+v", string(bts))
		var response struct {
			*mollie.Payment
			Links struct {
				Self *mollie.Link `json:"self,omitempty"`
				Checkout *mollie.Link `json:"checkout,omitempty"`
				Dashboard *mollie.Link `json:"dashboard,omitempty"`
				Documentation *mollie.Link `json:"documentation,omitempty"`
			} `json:"_links,omitempty"`
		}
		if err = json.Unmarshal(bts, &response); err != nil {
			return nil, err
		}
		return response.Payment, nil
	}else{
		return nil, err
	}
}