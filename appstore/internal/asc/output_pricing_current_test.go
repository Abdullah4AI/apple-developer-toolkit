package asc

import (
	"reflect"
	"testing"
)

func TestAppPriceScheduleNotConfiguredRows(t *testing.T) {
	headers, rows := appPriceScheduleNotConfiguredRows(&AppPriceScheduleNotConfiguredResult{AppID: "app-1"})
	if want := []string{"App ID", "Configured"}; !reflect.DeepEqual(headers, want) {
		t.Fatalf("headers = %v, want %v", headers, want)
	}
	if want := [][]string{{"app-1", "false"}}; !reflect.DeepEqual(rows, want) {
		t.Fatalf("rows = %v, want %v", rows, want)
	}
}

func TestAppPriceScheduleNotConfiguredResultIsRegistered(t *testing.T) {
	ensureOutputRegistryPopulated()
	if !isRegistryTypeRegistered(typeForPtr[AppPriceScheduleNotConfiguredResult]()) {
		t.Fatal("expected AppPriceScheduleNotConfiguredResult to have a registered renderer")
	}
}
