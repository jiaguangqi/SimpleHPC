package service

import (
	"strings"
	"testing"
)

func TestResetCodeHashBindsCodeToRequest(t *testing.T) {
	first := resetCodeHash("request-a", "123456")
	if first == resetCodeHash("request-b", "123456") {
		t.Fatal("same code must have a different hash for another request")
	}
	if first == resetCodeHash("request-a", "654321") {
		t.Fatal("different codes must not have the same hash")
	}
}

func TestRandomDigits(t *testing.T) {
	code, err := randomDigits(6)
	if err != nil {
		t.Fatal(err)
	}
	if len(code) != 6 {
		t.Fatalf("expected six digits, got %q", code)
	}
	if strings.Trim(code, "0123456789") != "" {
		t.Fatalf("code contains non-digits: %q", code)
	}
}

func TestNormalizeAccountType(t *testing.T) {
	if got := normalizeAccountType(" ADMIN "); got != "admin" {
		t.Fatalf("unexpected admin type: %q", got)
	}
	if got := normalizeAccountType("ldap"); got != "ldap" {
		t.Fatalf("unexpected ldap type: %q", got)
	}
	if got := normalizeAccountType("root"); got != "" {
		t.Fatalf("unsupported account type was accepted: %q", got)
	}
}
