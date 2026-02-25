package asc

// FinanceRegion describes a finance report region code and currency.
//
// Region codes are used with the --region flag for 'asc finance reports'.
// Special region codes:
//   - ZZ: Consolidated Financial Reports (all regions, use with FINANCIAL)
//   - Z1: Financial Detail Reports (all regions, required for FINANCE_DETAIL)
//   - EU, LL, AP, WW: Regional aggregates (use with FINANCIAL)
//   - US, AU, JP, etc.: Individual country reports (use with FINANCIAL)
//
// Reference: https://developer.apple.com/help/app-store-connect/reference/financial-report-regions-and-currencies/
type FinanceRegion struct {
	ReportRegion       string `json:"reportRegion"`
	ReportCurrency     string `json:"reportCurrency"`
	RegionCode         string `json:"regionCode"`
	CountriesOrRegions string `json:"countriesOrRegions"`
}

// FinanceRegionsResult represents CLI output for finance region listings.
type FinanceRegionsResult struct {
	Regions []FinanceRegion `json:"regions"`
}

var financeRegions = []FinanceRegion{
	{ReportRegion: "Americas", ReportCurrency: "USD", RegionCode: "US", CountriesOrRegions: "United States"},
	{ReportRegion: "Australia", ReportCurrency: "AUD", RegionCode: "AU", CountriesOrRegions: "Australia"},
	{ReportRegion: "Brazil", ReportCurrency: "BRL", RegionCode: "BR", CountriesOrRegions: "Brazil"},
	{ReportRegion: "Bulgaria", ReportCurrency: "BGN", RegionCode: "BG", CountriesOrRegions: "Bulgaria"},
	{ReportRegion: "Canada", ReportCurrency: "CAD", RegionCode: "CA", CountriesOrRegions: "Canada"},
	{ReportRegion: "Chile", ReportCurrency: "CLP", RegionCode: "CL", CountriesOrRegions: "Chile"},
	{ReportRegion: "China mainland", ReportCurrency: "CNY", RegionCode: "CN", CountriesOrRegions: "China mainland"},
	{ReportRegion: "Colombia", ReportCurrency: "COP", RegionCode: "CO", CountriesOrRegions: "Colombia"},
	{ReportRegion: "Czech Republic", ReportCurrency: "CZK", RegionCode: "CZ", CountriesOrRegions: "Czech Republic"},
	{ReportRegion: "Croatia", ReportCurrency: "EUR", RegionCode: "HR", CountriesOrRegions: "Croatia"},
	{ReportRegion: "Denmark", ReportCurrency: "DKK", RegionCode: "DK", CountriesOrRegions: "Denmark"},
	{ReportRegion: "Egypt", ReportCurrency: "EGP", RegionCode: "EG", CountriesOrRegions: "Egypt"},
	{ReportRegion: "Hong Kong", ReportCurrency: "HKD", RegionCode: "HK", CountriesOrRegions: "Hong Kong"},
	{ReportRegion: "Hungary", ReportCurrency: "HUF", RegionCode: "HU", CountriesOrRegions: "Hungary"},
	{ReportRegion: "India", ReportCurrency: "INR", RegionCode: "IN", CountriesOrRegions: "India"},
	{ReportRegion: "Indonesia", ReportCurrency: "IDR", RegionCode: "ID", CountriesOrRegions: "Indonesia"},
	{ReportRegion: "Israel", ReportCurrency: "ILS", RegionCode: "IL", CountriesOrRegions: "Israel"},
	{ReportRegion: "Japan", ReportCurrency: "JPY", RegionCode: "JP", CountriesOrRegions: "Japan"},
	{ReportRegion: "Kazakhstan", ReportCurrency: "KZT", RegionCode: "KZ", CountriesOrRegions: "Kazakhstan"},
	{ReportRegion: "Republic of Korea", ReportCurrency: "KRW", RegionCode: "KR", CountriesOrRegions: "Republic of Korea"},
	{ReportRegion: "Malaysia", ReportCurrency: "MYR", RegionCode: "MY", CountriesOrRegions: "Malaysia"},
	{ReportRegion: "Mexico", ReportCurrency: "MXN", RegionCode: "MX", CountriesOrRegions: "Mexico"},
	{ReportRegion: "New Zealand", ReportCurrency: "NZD", RegionCode: "NZ", CountriesOrRegions: "New Zealand"},
	{ReportRegion: "Nigeria", ReportCurrency: "NGN", RegionCode: "NG", CountriesOrRegions: "Nigeria"},
	{ReportRegion: "Norway", ReportCurrency: "NOK", RegionCode: "NO", CountriesOrRegions: "Norway"},
	{ReportRegion: "Pakistan", ReportCurrency: "PKR", RegionCode: "PK", CountriesOrRegions: "Pakistan"},
	{ReportRegion: "Peru", ReportCurrency: "PEN", RegionCode: "PE", CountriesOrRegions: "Peru"},
	{ReportRegion: "Philippines", ReportCurrency: "PHP", RegionCode: "PH", CountriesOrRegions: "Philippines"},
	{ReportRegion: "Poland", ReportCurrency: "PLN", RegionCode: "PL", CountriesOrRegions: "Poland"},
	{ReportRegion: "Qatar", ReportCurrency: "QAR", RegionCode: "QA", CountriesOrRegions: "Qatar"},
	{ReportRegion: "Romania", ReportCurrency: "RON", RegionCode: "RO", CountriesOrRegions: "Romania"},
	{ReportRegion: "Russia", ReportCurrency: "RUB", RegionCode: "RU", CountriesOrRegions: "Russia"},
	{ReportRegion: "Saudi Arabia", ReportCurrency: "SAR", RegionCode: "SA", CountriesOrRegions: "Saudi Arabia"},
	{ReportRegion: "Singapore", ReportCurrency: "SGD", RegionCode: "SG", CountriesOrRegions: "Singapore"},
	{ReportRegion: "South Africa", ReportCurrency: "ZAR", RegionCode: "ZA", CountriesOrRegions: "South Africa"},
	{ReportRegion: "Sweden", ReportCurrency: "SEK", RegionCode: "SE", CountriesOrRegions: "Sweden"},
	{ReportRegion: "Switzerland", ReportCurrency: "CHF", RegionCode: "CH", CountriesOrRegions: "Switzerland"},
	{ReportRegion: "Taiwan", ReportCurrency: "TWD", RegionCode: "TW", CountriesOrRegions: "Taiwan"},
	{ReportRegion: "Thailand", ReportCurrency: "THB", RegionCode: "TH", CountriesOrRegions: "Thailand"},
	{ReportRegion: "Turkiye", ReportCurrency: "TRY", RegionCode: "TR", CountriesOrRegions: "Turkiye"},
	{ReportRegion: "United Arab Emirates", ReportCurrency: "AED", RegionCode: "AE", CountriesOrRegions: "United Arab Emirates"},
	{ReportRegion: "United Kingdom", ReportCurrency: "GBP", RegionCode: "GB", CountriesOrRegions: "United Kingdom"},
	{ReportRegion: "Tanzania", ReportCurrency: "TZS", RegionCode: "TZ", CountriesOrRegions: "Tanzania"},
	{ReportRegion: "Vietnam", ReportCurrency: "VND", RegionCode: "VN", CountriesOrRegions: "Vietnam"},
	{ReportRegion: "Euro-Zone", ReportCurrency: "EUR", RegionCode: "EU", CountriesOrRegions: "Austria, Belgium, Bosnia and Herzegovina, Bulgaria, Cyprus, Czech Republic, Estonia, Finland, France, Germany, Greece, Hungary, Ireland, Italy, Latvia, Lithuania, Luxembourg, Malta, Montenegro, Netherlands, Poland, Portugal, Romania, Serbia, Slovakia, Slovenia, Spain"},
	{ReportRegion: "Latin America and the Caribbean", ReportCurrency: "USD", RegionCode: "LL", CountriesOrRegions: "Anguilla, Antigua and Barbuda, Argentina, Bahamas, Barbados, Belize, Bermuda, Bolivia, British Virgin Islands, Cayman Islands, Chile, Costa Rica, Dominica, Dominican Republic, Ecuador, El Salvador, Grenada, Guatemala, Guyana, Honduras, Jamaica, Montserrat, Nicaragua, Panama, Paraguay, Saint Lucia, St. Kitts and Nevis, St. Vincent and the Grenadines, Suriname, Trinidad and Tobago, Turks and Caicos Islands, Uruguay, and Venezuela"},
	{ReportRegion: "South Asia and Pacific", ReportCurrency: "USD", RegionCode: "AP", CountriesOrRegions: "Bhutan, Brunei, Cambodia, Fiji, Laos, Macau, Maldives, Micronesia, Mongolia, Myanmar, Nauru, Nepal, Palau, Papua New Guinea, Solomon Islands, Sri Lanka, Tonga, and Vanuatu"},
	{ReportRegion: "Rest of World", ReportCurrency: "USD", RegionCode: "WW", CountriesOrRegions: "Afghanistan, Albania, Algeria, Angola, Armenia, Azerbaijan, Bahrain, Belarus, Benin, Botswana, Burkina Faso, Cameroon, Cape Verde, Chad, Democratic Republic of the Congo, Republic of the Congo, Cote d'Ivoire, Croatia, Egypt, Eswatini, Gabon, Gambia, Georgia, Ghana, Guinea-Bissau, Iceland, Iraq, Jordan, Kazakhstan, Kenya, Kuwait, Kyrgyzstan, Lebanon, Liberia, Libya, Madagascar, Malawi, Malaysia, Mali, Mauritania, Mauritius, Moldova, Morocco, Mozambique, Namibia, Niger, Nigeria, North Macedonia, Oman, Pakistan, Philippines, Qatar, Rwanda, Sao Tome and Principe, Senegal, Seychelles, Sierra Leone, Tajikistan, Tunisia, Turkmenistan, Uganda, Ukraine, Uzbekistan, Yemen, Zambia, Zimbabwe"},
	{ReportRegion: "Consolidated Financial Reports", ReportCurrency: "Multiple", RegionCode: "ZZ", CountriesOrRegions: "All Countries or Regions"},
	{ReportRegion: "Financial Detail Reports", ReportCurrency: "Multiple", RegionCode: "Z1", CountriesOrRegions: "All Countries or Regions"},
}

// FinanceRegions returns a copy of the Apple finance report region codes list.
func FinanceRegions() []FinanceRegion {
	return append([]FinanceRegion(nil), financeRegions...)
}
