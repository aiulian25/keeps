package importer

import "testing"

func TestRobotsRules(t *testing.T) {
	robots := []byte("User-agent: *\nDisallow: /checkout\nDisallow: /*?session=\nAllow: /checkout/help\n\nUser-agent: OtherBot\nDisallow: /\n")
	cases := map[string]bool{
		"/product/42":        true,
		"/checkout/cart":     false,
		"/checkout/help/faq": true,
		"/p?session=abc":     false,
		"/p?colour=red":      true,
	}
	for path, want := range cases {
		if got := robotsPermits(robots, path); got != want {
			t.Errorf("%s: got %v want %v", path, got, want)
		}
	}
	keepsSpecific := []byte("User-agent: *\nDisallow: /\n\nUser-agent: Keeps\nAllow: /\n")
	if !robotsPermits(keepsSpecific, "/anything") {
		t.Error("specific Keeps group should override *")
	}
	if !robotsPermits([]byte("User-agent: *\nDisallow:\n"), "/x") {
		t.Error("empty disallow should allow everything")
	}
	if robotsPermits([]byte("User-agent: *\nDisallow: /*.b$\n"), "/a.b.c.b") {
		t.Error("anchored wildcard should match the final occurrence")
	}
	if !robotsPermits([]byte("User-agent: *\nDisallow: /*.pdf$\n"), "/files/invoice.pdf?x=1") {
		t.Error("anchor should not match when text follows")
	}
	if robotsPermits([]byte("User-agent: *\nDisallow: /*.pdf$\n"), "/files/invoice.pdf") {
		t.Error("anchored wildcard should match")
	}
}
