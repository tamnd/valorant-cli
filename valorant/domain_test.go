package valorant

import (
	"testing"

	"github.com/tamnd/any-cli/kit"
)

// These tests are offline: they exercise the URI driver's pure string functions
// and the host wiring, which need no network.

func TestDomainInfo(t *testing.T) {
	info := Domain{}.Info()
	if info.Scheme != "valorant" {
		t.Errorf("Scheme = %q, want valorant", info.Scheme)
	}
	if len(info.Hosts) == 0 || info.Hosts[0] != Host {
		t.Errorf("Hosts = %v, want [%s]", info.Hosts, Host)
	}
	if info.Identity.Binary != "valorant" {
		t.Errorf("Identity.Binary = %q, want valorant", info.Identity.Binary)
	}
}

func TestClassify(t *testing.T) {
	cases := []struct {
		in  string
		typ string
		id  string
	}{
		{"valorant://agents/abc", "agents", "abc"},
		{"valorant://maps/xyz", "maps", "xyz"},
		{"valorant://weapons/def", "weapons", "def"},
	}
	for _, tc := range cases {
		typ, id, err := Domain{}.Classify(tc.in)
		if err != nil || typ != tc.typ || id != tc.id {
			t.Errorf("Classify(%q) = (%q, %q, %v), want (%q, %q, nil)",
				tc.in, typ, id, err, tc.typ, tc.id)
		}
	}
}

func TestLocate(t *testing.T) {
	cases := []struct {
		typ  string
		id   string
		want string
	}{
		{"agents", "abc-uuid", "https://valorant-api.com/v1/agents/abc-uuid"},
		{"maps", "map-uuid", "https://valorant-api.com/v1/maps/map-uuid"},
		{"weapons", "gun-uuid", "https://valorant-api.com/v1/weapons/gun-uuid"},
	}
	for _, tc := range cases {
		got, err := Domain{}.Locate(tc.typ, tc.id)
		if err != nil || got != tc.want {
			t.Errorf("Locate(%q, %q) = (%q, %v), want (%q, nil)", tc.typ, tc.id, got, err, tc.want)
		}
	}
}

func TestDomainRegistered(t *testing.T) {
	h, err := kit.Open()
	if err != nil {
		t.Fatal(err)
	}
	domains := h.Domains()
	found := false
	for _, d := range domains {
		if d == "valorant" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("valorant domain not registered; domains = %v", domains)
	}
}
