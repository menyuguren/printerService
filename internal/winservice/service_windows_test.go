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

func TestServiceMainMatchesWindowsCallbackSignature(t *testing.T) {
	typ := reflect.TypeOf((*serviceHandler).serviceMain)

	if typ.NumIn() != 3 {
		t.Fatalf("serviceMain() has %d inputs, want 3", typ.NumIn())
	}
	if typ.NumOut() != 1 {
		t.Fatalf("serviceMain() has %d outputs, want 1", typ.NumOut())
	}
	if typ.Out(0).Kind() != reflect.Uintptr {
		t.Fatalf("serviceMain() returns %s, want uintptr", typ.Out(0).Kind())
	}
}
