package cmdtest

import "testing"

func TestIAPListPaginateFromNextWithoutAppPhase61(t *testing.T) {
	const firstURL = "https://api.appstoreconnect.apple.com/v1/apps/app-1/inAppPurchasesV2?cursor=AQ&limit=200"
	const secondURL = "https://api.appstoreconnect.apple.com/v1/apps/app-1/inAppPurchasesV2?cursor=BQ&limit=200"

	firstBody := `{"data":[{"type":"inAppPurchases","id":"iap-next-1"}],"links":{"next":"` + secondURL + `"}}`
	secondBody := `{"data":[{"type":"inAppPurchases","id":"iap-next-2"}],"links":{"next":""}}`

	runGameCenterAchievementsPaginateFromNext(
		t,
		[]string{"iap", "list"},
		firstURL,
		secondURL,
		firstBody,
		secondBody,
		"iap-next-1",
		"iap-next-2",
	)
}

func TestIAPOfferCodeCustomCodesListRejectsInvalidNextURLPhase61(t *testing.T) {
	runGameCenterAchievementsInvalidNextURLCases(
		t,
		[]string{"iap", "offer-codes", "custom-codes", "list"},
		"iap offer-codes custom-codes list: --next",
	)
}

func TestIAPOfferCodeCustomCodesListPaginateFromNextWithoutOfferCodeIDPhase61(t *testing.T) {
	const firstURL = "https://api.appstoreconnect.apple.com/v1/inAppPurchaseOfferCodes/offer-1/customCodes?cursor=AQ&limit=200"
	const secondURL = "https://api.appstoreconnect.apple.com/v1/inAppPurchaseOfferCodes/offer-1/customCodes?cursor=BQ&limit=200"

	firstBody := `{"data":[{"type":"inAppPurchaseOfferCodeCustomCodes","id":"iap-custom-code-next-1"}],"links":{"next":"` + secondURL + `"}}`
	secondBody := `{"data":[{"type":"inAppPurchaseOfferCodeCustomCodes","id":"iap-custom-code-next-2"}],"links":{"next":""}}`

	runGameCenterAchievementsPaginateFromNext(
		t,
		[]string{"iap", "offer-codes", "custom-codes", "list"},
		firstURL,
		secondURL,
		firstBody,
		secondBody,
		"iap-custom-code-next-1",
		"iap-custom-code-next-2",
	)
}

func TestIAPOfferCodeOneTimeCodesListRejectsInvalidNextURLPhase61(t *testing.T) {
	runGameCenterAchievementsInvalidNextURLCases(
		t,
		[]string{"iap", "offer-codes", "one-time-codes", "list"},
		"iap offer-codes one-time-codes list: --next",
	)
}

func TestIAPOfferCodeOneTimeCodesListPaginateFromNextWithoutOfferCodeIDPhase61(t *testing.T) {
	const firstURL = "https://api.appstoreconnect.apple.com/v1/inAppPurchaseOfferCodes/offer-1/oneTimeUseCodes?cursor=AQ&limit=200"
	const secondURL = "https://api.appstoreconnect.apple.com/v1/inAppPurchaseOfferCodes/offer-1/oneTimeUseCodes?cursor=BQ&limit=200"

	firstBody := `{"data":[{"type":"inAppPurchaseOfferCodeOneTimeUseCodes","id":"iap-one-time-code-next-1"}],"links":{"next":"` + secondURL + `"}}`
	secondBody := `{"data":[{"type":"inAppPurchaseOfferCodeOneTimeUseCodes","id":"iap-one-time-code-next-2"}],"links":{"next":""}}`

	runGameCenterAchievementsPaginateFromNext(
		t,
		[]string{"iap", "offer-codes", "one-time-codes", "list"},
		firstURL,
		secondURL,
		firstBody,
		secondBody,
		"iap-one-time-code-next-1",
		"iap-one-time-code-next-2",
	)
}

func TestIAPOfferCodePricesPaginateFromNextWithoutOfferCodeIDPhase61(t *testing.T) {
	const firstURL = "https://api.appstoreconnect.apple.com/v1/inAppPurchaseOfferCodes/offer-1/prices?cursor=AQ&fields%5BinAppPurchaseOfferPrices%5D=territory%2CpricePoint&include=territory%2CpricePoint&limit=200"
	const secondURL = "https://api.appstoreconnect.apple.com/v1/inAppPurchaseOfferCodes/offer-1/prices?cursor=BQ&fields%5BinAppPurchaseOfferPrices%5D=territory%2CpricePoint&include=territory%2CpricePoint&limit=200"

	firstBody := `{"data":[{"type":"inAppPurchaseOfferPrices","id":"iap-offer-price-next-1"}],"links":{"next":"` + secondURL + `"}}`
	secondBody := `{"data":[{"type":"inAppPurchaseOfferPrices","id":"iap-offer-price-next-2"}],"links":{"next":""}}`

	runGameCenterAchievementsPaginateFromNext(
		t,
		[]string{"iap", "offer-codes", "prices"},
		firstURL,
		secondURL,
		firstBody,
		secondBody,
		"iap-offer-price-next-1",
		"iap-offer-price-next-2",
	)
}

func TestIAPPricePointsListRejectsInvalidNextURLPhase61(t *testing.T) {
	runGameCenterAchievementsInvalidNextURLCases(
		t,
		[]string{"iap", "pricing", "price-points", "list"},
		"iap price-points list: --next",
	)
}

func TestIAPPricePointsListPaginateFromNextWithoutIAPIDPhase61(t *testing.T) {
	const firstURL = "https://api.appstoreconnect.apple.com/v2/inAppPurchases/iap-1/pricePoints?cursor=AQ&limit=200"
	const secondURL = "https://api.appstoreconnect.apple.com/v2/inAppPurchases/iap-1/pricePoints?cursor=BQ&limit=200"

	firstBody := `{"data":[{"type":"inAppPurchasePricePoints","id":"iap-price-point-next-1"}],"links":{"next":"` + secondURL + `"}}`
	secondBody := `{"data":[{"type":"inAppPurchasePricePoints","id":"iap-price-point-next-2"}],"links":{"next":""}}`

	runGameCenterAchievementsPaginateFromNext(
		t,
		[]string{"iap", "pricing", "price-points", "list"},
		firstURL,
		secondURL,
		firstBody,
		secondBody,
		"iap-price-point-next-1",
		"iap-price-point-next-2",
	)
}

func TestIAPPriceSchedulesAutomaticPricesRejectsInvalidNextURLPhase61(t *testing.T) {
	runGameCenterAchievementsInvalidNextURLCases(
		t,
		[]string{"iap", "pricing", "schedules", "automatic-prices"},
		"iap pricing schedules automatic-prices: --next",
	)
}

func TestIAPPriceSchedulesAutomaticPricesPaginateFromNextWithoutScheduleIDPhase61(t *testing.T) {
	const firstURL = "https://api.appstoreconnect.apple.com/v1/inAppPurchasePriceSchedules/schedule-1/automaticPrices?cursor=AQ&limit=200"
	const secondURL = "https://api.appstoreconnect.apple.com/v1/inAppPurchasePriceSchedules/schedule-1/automaticPrices?cursor=BQ&limit=200"

	firstBody := `{"data":[{"type":"inAppPurchasePrices","id":"iap-automatic-price-next-1"}],"links":{"next":"` + secondURL + `"}}`
	secondBody := `{"data":[{"type":"inAppPurchasePrices","id":"iap-automatic-price-next-2"}],"links":{"next":""}}`

	runGameCenterAchievementsPaginateFromNext(
		t,
		[]string{"iap", "pricing", "schedules", "automatic-prices"},
		firstURL,
		secondURL,
		firstBody,
		secondBody,
		"iap-automatic-price-next-1",
		"iap-automatic-price-next-2",
	)
}

func TestIAPPriceSchedulesManualPricesRejectsInvalidNextURLPhase61(t *testing.T) {
	runGameCenterAchievementsInvalidNextURLCases(
		t,
		[]string{"iap", "pricing", "schedules", "manual-prices"},
		"iap pricing schedules manual-prices: --next",
	)
}

func TestIAPPriceSchedulesManualPricesPaginateFromNextWithoutScheduleIDPhase61(t *testing.T) {
	const firstURL = "https://api.appstoreconnect.apple.com/v1/inAppPurchasePriceSchedules/schedule-1/manualPrices?cursor=AQ&limit=200"
	const secondURL = "https://api.appstoreconnect.apple.com/v1/inAppPurchasePriceSchedules/schedule-1/manualPrices?cursor=BQ&limit=200"

	firstBody := `{"data":[{"type":"inAppPurchasePrices","id":"iap-manual-price-next-1"}],"links":{"next":"` + secondURL + `"}}`
	secondBody := `{"data":[{"type":"inAppPurchasePrices","id":"iap-manual-price-next-2"}],"links":{"next":""}}`

	runGameCenterAchievementsPaginateFromNext(
		t,
		[]string{"iap", "pricing", "schedules", "manual-prices"},
		firstURL,
		secondURL,
		firstBody,
		secondBody,
		"iap-manual-price-next-1",
		"iap-manual-price-next-2",
	)
}

func TestIAPAvailabilityAvailableTerritoriesRejectsInvalidNextURLPhase61(t *testing.T) {
	runGameCenterAchievementsInvalidNextURLCases(
		t,
		[]string{"iap", "pricing", "availabilities", "available-territories"},
		"iap availabilities available-territories: --next",
	)
}

func TestIAPAvailabilityAvailableTerritoriesPaginateFromNextWithoutAvailabilityIDPhase61(t *testing.T) {
	const firstURL = "https://api.appstoreconnect.apple.com/v1/inAppPurchaseAvailabilities/availability-1/availableTerritories?cursor=AQ&limit=200"
	const secondURL = "https://api.appstoreconnect.apple.com/v1/inAppPurchaseAvailabilities/availability-1/availableTerritories?cursor=BQ&limit=200"

	firstBody := `{"data":[{"type":"territories","id":"territory-next-1"}],"links":{"next":"` + secondURL + `"}}`
	secondBody := `{"data":[{"type":"territories","id":"territory-next-2"}],"links":{"next":""}}`

	runGameCenterAchievementsPaginateFromNext(
		t,
		[]string{"iap", "pricing", "availabilities", "available-territories"},
		firstURL,
		secondURL,
		firstBody,
		secondBody,
		"territory-next-1",
		"territory-next-2",
	)
}
