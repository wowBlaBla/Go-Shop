package mollie

type Line struct {
	Type string `json:"type,omitempty"` // 'physical' (default), 'discount', 'digital', 'shipping_fee', 'store_credit', 'gift_card', 'surcharge'
	Category string `json:"category,omitempty"` // 'meal', 'eco', 'gift'
	Sku string `json:"sku,omitempty"`
	Name string `json:"name,omitempty"`
	ProductUrl string `json:"productUrl,omitempty"`
	ImageUrl string `json:"imageUrl,omitempty"`
	Metadata map[string]string `json:"metadata,omitempty"`
	Quantity int `json:"quantity,omitempty"`
	VatRate string `json:"vatRate,omitempty"`
	UnitPrice Amount `json:"unitPrice,omitempty"`
	TotalAmount Amount `json:"totalAmount,omitempty"`
	DiscountAmount Amount `json:"discountAmount,omitempty"`
	VatAmount Amount `json:"vatAmount,omitempty"`
}