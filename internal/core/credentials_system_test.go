package core

import (
	"os"
	"runtime"
	"testing"
)

// Explicit integration check: normal unit tests use memorySecrets and never
// change the host credential store. This check creates and removes only a new
// random test credential and does not read existing user entries.
func TestSystemCredentialRoundTrip(t *testing.T) {
	if os.Getenv("SMARTNOTE_TEST_OS_CREDENTIALS") != "1" || runtime.GOOS != "windows" {
		t.Skip("opt-in Windows credential integration check")
	}
	store := systemSecrets{}
	key := "isolated-test:" + uid()
	value := "non-sensitive-integration-fixture"
	if e := store.Set(key, value); e != nil {
		t.Fatal(e)
	}
	defer store.Delete(key)
	got, e := store.Get(key)
	if e != nil || got != value {
		t.Fatalf("credential roundtrip failed: %v", e)
	}
	if e = store.Delete(key); e != nil {
		t.Fatal(e)
	}
	got, e = store.Get(key)
	if e != nil || got != "" {
		t.Fatalf("credential deletion failed: %v", e)
	}
}
