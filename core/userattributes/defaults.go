package userattributes

// Spec is a system attribute definition seeded per organisation.
type Spec struct {
	Key         string
	Label       string
	Description string
	ValueType   string
	Section     string
	SortOrder   int32
	Required    bool
	EnumValues  []string
	IsPII       bool
}

// Defaults returns built-in attribute definitions seeded for each organisation.
// display_name / email stay first-class AppUser columns and are not included.
func Defaults() []Spec {
	return []Spec{
		{
			Key: "phone", Label: "Mobile", Description: "Mobile phone number",
			ValueType: "string", Section: "contact", SortOrder: 10, IsPII: true,
		},
		{
			Key: "location", Label: "Location", Description: "City or region",
			ValueType: "string", Section: "profile", SortOrder: 10,
		},
		{
			Key: "country", Label: "Country", Description: "Country of residence",
			ValueType: "dropdown", Section: "profile", SortOrder: 20,
			EnumValues: commonCountries(),
		},
		{
			Key: "date_of_birth", Label: "Date of birth",
			ValueType: "date", Section: "profile", SortOrder: 30, IsPII: true,
		},
		{
			Key: "address_line1", Label: "Address", Description: "Street address",
			ValueType: "string", Section: "address", SortOrder: 10, IsPII: true,
		},
		{
			Key: "city", Label: "City",
			ValueType: "string", Section: "address", SortOrder: 20,
		},
		{
			Key: "postal_code", Label: "Postal code",
			ValueType: "string", Section: "address", SortOrder: 30,
		},
	}
}

func commonCountries() []string {
	return []string{
		"AU", "NZ", "US", "GB", "CA", "SG", "MY", "IN", "ID", "PH",
		"TH", "VN", "JP", "KR", "CN", "HK", "TW", "DE", "FR", "NL",
		"AE", "SA", "ZA", "BR", "MX",
	}
}
