package cmdtest

import "testing"

func TestGameCenterAppVersionsListRejectsInvalidNextURL(t *testing.T) {
	runGameCenterAchievementsInvalidNextURLCases(
		t,
		[]string{"game-center", "app-versions", "list"},
		"game-center app-versions list: --next",
	)
}

func TestGameCenterAppVersionsListPaginateFromNextWithoutApp(t *testing.T) {
	const firstURL = "https://api.appstoreconnect.apple.com/v1/gameCenterDetails/gc-detail-1/gameCenterAppVersions?cursor=AQ&limit=200"
	const secondURL = "https://api.appstoreconnect.apple.com/v1/gameCenterDetails/gc-detail-1/gameCenterAppVersions?cursor=BQ&limit=200"

	firstBody := `{"data":[{"type":"gameCenterAppVersions","id":"gc-app-version-next-1"}],"links":{"next":"` + secondURL + `"}}`
	secondBody := `{"data":[{"type":"gameCenterAppVersions","id":"gc-app-version-next-2"}],"links":{"next":""}}`

	runGameCenterAchievementsPaginateFromNext(
		t,
		[]string{"game-center", "app-versions", "list"},
		firstURL,
		secondURL,
		firstBody,
		secondBody,
		"gc-app-version-next-1",
		"gc-app-version-next-2",
	)
}

func TestGameCenterAppVersionCompatibilityListPaginateFromNextWithoutID(t *testing.T) {
	const firstURL = "https://api.appstoreconnect.apple.com/v1/gameCenterAppVersions/gcav-1/compatibilityVersions?cursor=AQ&limit=200"
	const secondURL = "https://api.appstoreconnect.apple.com/v1/gameCenterAppVersions/gcav-1/compatibilityVersions?cursor=BQ&limit=200"

	firstBody := `{"data":[{"type":"gameCenterAppVersions","id":"gc-app-version-compatibility-next-1"}],"links":{"next":"` + secondURL + `"}}`
	secondBody := `{"data":[{"type":"gameCenterAppVersions","id":"gc-app-version-compatibility-next-2"}],"links":{"next":""}}`

	runGameCenterAchievementsPaginateFromNext(
		t,
		[]string{"game-center", "app-versions", "compatibility", "list"},
		firstURL,
		secondURL,
		firstBody,
		secondBody,
		"gc-app-version-compatibility-next-1",
		"gc-app-version-compatibility-next-2",
	)
}

func TestGameCenterEnabledVersionsListRejectsInvalidNextURL(t *testing.T) {
	runGameCenterAchievementsInvalidNextURLCases(
		t,
		[]string{"game-center", "enabled-versions", "list"},
		"game-center enabled-versions list: --next",
	)
}

func TestGameCenterEnabledVersionsListPaginateFromNextWithoutApp(t *testing.T) {
	const firstURL = "https://api.appstoreconnect.apple.com/v1/apps/app-1/gameCenterEnabledVersions?cursor=AQ&limit=200"
	const secondURL = "https://api.appstoreconnect.apple.com/v1/apps/app-1/gameCenterEnabledVersions?cursor=BQ&limit=200"

	firstBody := `{"data":[{"type":"gameCenterEnabledVersions","id":"gc-enabled-version-next-1"}],"links":{"next":"` + secondURL + `"}}`
	secondBody := `{"data":[{"type":"gameCenterEnabledVersions","id":"gc-enabled-version-next-2"}],"links":{"next":""}}`

	runGameCenterAchievementsPaginateFromNext(
		t,
		[]string{"game-center", "enabled-versions", "list"},
		firstURL,
		secondURL,
		firstBody,
		secondBody,
		"gc-enabled-version-next-1",
		"gc-enabled-version-next-2",
	)
}

func TestGameCenterEnabledVersionsCompatibleVersionsRejectsInvalidNextURL(t *testing.T) {
	runGameCenterAchievementsInvalidNextURLCases(
		t,
		[]string{"game-center", "enabled-versions", "compatible-versions"},
		"game-center enabled-versions compatible-versions: --next",
	)
}

func TestGameCenterEnabledVersionsCompatibleVersionsPaginateFromNextWithoutID(t *testing.T) {
	const firstURL = "https://api.appstoreconnect.apple.com/v1/gameCenterEnabledVersions/enabled-1/compatibleVersions?cursor=AQ&limit=200"
	const secondURL = "https://api.appstoreconnect.apple.com/v1/gameCenterEnabledVersions/enabled-1/compatibleVersions?cursor=BQ&limit=200"

	firstBody := `{"data":[{"type":"gameCenterEnabledVersions","id":"gc-enabled-version-compatibility-next-1"}],"links":{"next":"` + secondURL + `"}}`
	secondBody := `{"data":[{"type":"gameCenterEnabledVersions","id":"gc-enabled-version-compatibility-next-2"}],"links":{"next":""}}`

	runGameCenterAchievementsPaginateFromNext(
		t,
		[]string{"game-center", "enabled-versions", "compatible-versions"},
		firstURL,
		secondURL,
		firstBody,
		secondBody,
		"gc-enabled-version-compatibility-next-1",
		"gc-enabled-version-compatibility-next-2",
	)
}
