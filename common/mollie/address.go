package mollie

type Address struct {
	OrganizationName string `json:"organizationName,omitempty"`
	StreetAndNumber string `json:"streetAndNumber,omitempty"`
	City string `json:"city,omitempty"`
	Region string `json:"region,omitempty"`
	PostalCode string `json:"postalCode,omitempty"`
	Country string `json:"country,omitempty"` // 'DE'
	Title string `json:"title,omitempty"` // 'Dhr'
	GivenName string `json:"givenName,omitempty"`
	FamilyName string `json:"familyName,omitempty"`
	Email string `json:"email,omitempty"`
	Phone string `json:"phone,omitempty"`
}