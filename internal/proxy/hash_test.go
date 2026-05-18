package proxy

import "testing"

func TestHashUserDeterministic(t *testing.T) {
	a := hashUser("alice", "my-salt")
	b := hashUser("alice", "my-salt")
	if a != b {
		t.Errorf("hashUser not deterministic: %q != %q", a, b)
	}
}

func TestHashUserDifferentUsers(t *testing.T) {
	a := hashUser("alice", "salt")
	b := hashUser("bob", "salt")
	if a == b {
		t.Error("different users produced the same hash")
	}
}

func TestHashUserSaltChangesOutput(t *testing.T) {
	a := hashUser("alice", "salt-a")
	b := hashUser("alice", "salt-b")
	if a == b {
		t.Error("different salts produced the same hash")
	}
}

func TestHashUserEmptySalt(t *testing.T) {
	h := hashUser("alice", "")
	if h == "" || h == "alice" {
		t.Errorf("unexpected hash with empty salt: %q", h)
	}
}

func TestHashUserLength(t *testing.T) {
	h := hashUser("alice", "salt")
	if len(h) != 64 { // SHA-256 = 32 bytes = 64 hex chars
		t.Errorf("expected 64 hex chars, got %d: %q", len(h), h)
	}
}
