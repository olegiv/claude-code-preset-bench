package auth

import (
	"strings"
	"testing"
)

func TestParseAndUser(t *testing.T) {
	k, err := Parse(strings.NewReader("# keys\n\nannkey-0123456789:ann\n bobkey-0123456789 : bob \n"))
	if err != nil {
		t.Fatal(err)
	}
	for key, want := range map[string]string{"annkey-0123456789": "ann", "bobkey-0123456789": "bob"} {
		if got, ok := k.User(key); !ok || got != want {
			t.Errorf("User(%q) = %q, %v", key, got, ok)
		}
	}
	if _, ok := k.User("nobody-0123456789"); ok {
		t.Error("an unknown key was accepted")
	}
	if _, ok := k.User(""); ok {
		t.Error("an empty key was accepted")
	}
}

func TestParseRejects(t *testing.T) {
	for _, in := range []string{"no-separator-0123456789", "annkey-0123456789:", "short:ann"} {
		if _, err := Parse(strings.NewReader(in)); err == nil {
			t.Errorf("Parse(%q) succeeded", in)
		}
	}
}
