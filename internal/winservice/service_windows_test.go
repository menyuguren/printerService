package winservice

import (
	"reflect"
	"testing"
)

func TestServiceArgsIncludesInternalServiceCommand(t *testing.T) {
	got := serviceArgs(`D:\printer\config.json`)
	want := []string{"service", "-config", `D:\printer\config.json`}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("serviceArgs() = %#v, want %#v", got, want)
	}
}
