package safefetch

import "net/netip"

// nonPublicPrefixes is the single source of truth for destinations Keeps
// refuses to contact. Sources:
//   - IANA IPv4 Special-Purpose Address Registry
//     https://www.iana.org/assignments/iana-ipv4-special-registry
//   - IANA IPv6 Special-Purpose Address Registry
//     https://www.iana.org/assignments/iana-ipv6-special-registry
//   - IANA multicast registries (224.0.0.0/4, ff00::/8)
//
// Translation and tunnelling prefixes (NAT64, 6to4, Teredo) are rejected
// outright because they can embed an internal IPv4 target. IPv4-mapped IPv6
// is unmapped before the lookup so the IPv4 rows apply to it.
var nonPublicPrefixes = []netip.Prefix{
	netip.MustParsePrefix("0.0.0.0/8"),       // "this network"
	netip.MustParsePrefix("10.0.0.0/8"),      // private-use
	netip.MustParsePrefix("100.64.0.0/10"),   // shared address space (CGNAT)
	netip.MustParsePrefix("127.0.0.0/8"),     // loopback
	netip.MustParsePrefix("169.254.0.0/16"),  // link-local, cloud metadata
	netip.MustParsePrefix("172.16.0.0/12"),   // private-use
	netip.MustParsePrefix("192.0.0.0/24"),    // IETF protocol assignments
	netip.MustParsePrefix("192.0.2.0/24"),    // documentation TEST-NET-1
	netip.MustParsePrefix("192.88.99.0/24"),  // deprecated 6to4 relay anycast
	netip.MustParsePrefix("192.168.0.0/16"),  // private-use
	netip.MustParsePrefix("198.18.0.0/15"),   // benchmarking
	netip.MustParsePrefix("198.51.100.0/24"), // documentation TEST-NET-2
	netip.MustParsePrefix("203.0.113.0/24"),  // documentation TEST-NET-3
	netip.MustParsePrefix("224.0.0.0/4"),     // multicast
	netip.MustParsePrefix("240.0.0.0/4"),     // reserved, includes 255.255.255.255 broadcast

	netip.MustParsePrefix("::/128"),         // unspecified
	netip.MustParsePrefix("::1/128"),        // loopback
	netip.MustParsePrefix("::ffff:0:0/96"),  // IPv4-mapped (defence in depth; unmapped first)
	netip.MustParsePrefix("64:ff9b::/96"),   // NAT64 well-known prefix
	netip.MustParsePrefix("64:ff9b:1::/48"), // NAT64 local-use
	netip.MustParsePrefix("100::/64"),       // discard-only
	netip.MustParsePrefix("2001::/32"),      // Teredo
	netip.MustParsePrefix("2001:2::/48"),    // benchmarking
	netip.MustParsePrefix("2001:10::/28"),   // deprecated ORCHID
	netip.MustParsePrefix("2001:db8::/32"),  // documentation
	netip.MustParsePrefix("2002::/16"),      // 6to4
	netip.MustParsePrefix("3fff::/20"),      // documentation
	netip.MustParsePrefix("fc00::/7"),       // unique-local
	netip.MustParsePrefix("fe80::/10"),      // link-local
	netip.MustParsePrefix("fec0::/10"),      // deprecated site-local
	netip.MustParsePrefix("ff00::/8"),       // multicast
}

// globalUnicastIPv6 is the only IPv6 block IANA allocates for global unicast;
// everything outside it is refused even if the table above misses it.
var globalUnicastIPv6 = netip.MustParsePrefix("2000::/3")

func isPublicAddress(address netip.Addr) bool {
	address = address.Unmap()
	if !address.IsValid() || address.Zone() != "" {
		return false
	}
	if address.Is6() && !globalUnicastIPv6.Contains(address) {
		return false
	}
	for _, prefix := range nonPublicPrefixes {
		if prefix.Contains(address) {
			return false
		}
	}
	return true
}
