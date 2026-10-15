package middleware

import (
	"net/http"
	"net/http/httptest"
	"net/netip"
	"testing"
)

// The addresses below come from the documentation ranges (RFC 5737 and
// RFC 3849). trustedProxies plays the role of the deploy platform's proxies:
// 10.0.0.0/8 for IPv4 and 2001:db8:ffff::/48 for IPv6.
var trustedProxies = []netip.Prefix{
	netip.MustParsePrefix("10.0.0.0/8"),
	netip.MustParsePrefix("2001:db8:ffff::/48"),
}

// newClientIPRequest builds a request with the given peer address and
// forwarding headers. Each xff element becomes its own header line.
func newClientIPRequest(remoteAddr string, xff []string, realIP string) *http.Request {
	req := httptest.NewRequest(http.MethodPost, "/tickets", nil)
	req.RemoteAddr = remoteAddr

	for _, line := range xff {
		req.Header.Add("X-Forwarded-For", line)
	}

	if realIP != "" {
		req.Header.Set("X-Real-IP", realIP)
	}

	return req
}

func TestClientIP(t *testing.T) {
	tests := []struct {
		name       string
		remoteAddr string
		xff        []string
		realIP     string
		trusted    []netip.Prefix
		want       string
	}{
		// RemoteAddr is the source by default.
		{
			name:       "IPv4 RemoteAddr",
			remoteAddr: "192.0.2.1:54321",
			want:       "192.0.2.1",
		},
		{
			name:       "IPv6 RemoteAddr",
			remoteAddr: "[2001:db8::1]:54321",
			want:       "2001:db8::1",
		},
		{
			name:       "IPv6 RemoteAddr with zone drops the zone",
			remoteAddr: "[fe80::1%eth0]:54321",
			want:       "fe80::1",
		},
		{
			name:       "RemoteAddr without port",
			remoteAddr: "192.0.2.1",
			want:       "192.0.2.1",
		},

		// Forwarding headers are ignored unless the peer is a trusted proxy.
		{
			name:       "X-Forwarded-For ignored when peer is not trusted",
			remoteAddr: "192.0.2.1:54321",
			xff:        []string{"203.0.113.7"},
			trusted:    trustedProxies,
			want:       "192.0.2.1",
		},
		{
			name:       "X-Forwarded-For ignored with nil trusted list",
			remoteAddr: "10.0.0.1:443",
			xff:        []string{"203.0.113.7"},
			want:       "10.0.0.1",
		},
		{
			name:       "X-Forwarded-For ignored with empty trusted list",
			remoteAddr: "10.0.0.1:443",
			xff:        []string{"203.0.113.7"},
			trusted:    []netip.Prefix{},
			want:       "10.0.0.1",
		},
		{
			name:       "trusted peer without X-Forwarded-For",
			remoteAddr: "10.0.0.1:443",
			trusted:    trustedProxies,
			want:       "10.0.0.1",
		},

		// Chains through trusted proxies.
		{
			name:       "single hop behind trusted proxy",
			remoteAddr: "10.0.0.1:443",
			xff:        []string{"203.0.113.7"},
			trusted:    trustedProxies,
			want:       "203.0.113.7",
		},
		{
			name:       "several trusted proxies are skipped",
			remoteAddr: "10.0.0.3:443",
			xff:        []string{"203.0.113.7, 10.0.0.1,10.0.0.2"},
			trusted:    trustedProxies,
			want:       "203.0.113.7",
		},
		{
			name:       "chain split across header lines",
			remoteAddr: "10.0.0.2:443",
			xff:        []string{"203.0.113.7", "10.0.0.1"},
			trusted:    trustedProxies,
			want:       "203.0.113.7",
		},
		{
			name:       "IPv6 chain behind trusted IPv6 proxy",
			remoteAddr: "[2001:db8:ffff::2]:443",
			xff:        []string{"2001:db8:1::5, 2001:db8:ffff::1"},
			trusted:    trustedProxies,
			want:       "2001:db8:1::5",
		},
		{
			name:       "every entry trusted returns the leftmost hop",
			remoteAddr: "10.0.0.2:443",
			xff:        []string{"10.0.0.5, 10.0.0.1"},
			trusted:    trustedProxies,
			want:       "10.0.0.5",
		},
		{
			name:       "entry with port is accepted",
			remoteAddr: "10.0.0.1:443",
			xff:        []string{"203.0.113.7:8080"},
			trusted:    trustedProxies,
			want:       "203.0.113.7",
		},

		// Spoofing: anything left of the first untrusted address is
		// client-controlled and must never win.
		{
			name:       "forged address on the left is ignored",
			remoteAddr: "10.0.0.1:443",
			xff:        []string{"198.51.100.66, 203.0.113.7"},
			trusted:    trustedProxies,
			want:       "203.0.113.7",
		},
		{
			name:       "forged trusted address on the left is ignored",
			remoteAddr: "10.0.0.1:443",
			xff:        []string{"10.0.0.9, 203.0.113.7"},
			trusted:    trustedProxies,
			want:       "203.0.113.7",
		},
		{
			name:       "forged header line before the proxy's line is ignored",
			remoteAddr: "10.0.0.1:443",
			xff:        []string{"198.51.100.66", "203.0.113.7"},
			trusted:    trustedProxies,
			want:       "203.0.113.7",
		},

		// Malformed entries stop the walk at the last trusted hop.
		{
			name:       "malformed header falls back to the trusted peer",
			remoteAddr: "10.0.0.1:443",
			xff:        []string{"not-an-ip"},
			trusted:    trustedProxies,
			want:       "10.0.0.1",
		},
		{
			name:       "empty header falls back to the trusted peer",
			remoteAddr: "10.0.0.1:443",
			xff:        []string{""},
			trusted:    trustedProxies,
			want:       "10.0.0.1",
		},
		{
			name:       "malformed rightmost entry hides valid entries to its left",
			remoteAddr: "10.0.0.1:443",
			xff:        []string{"203.0.113.7, garbage"},
			trusted:    trustedProxies,
			want:       "10.0.0.1",
		},
		{
			name:       "empty entry stops at the last trusted hop",
			remoteAddr: "10.0.0.2:443",
			xff:        []string{"203.0.113.7,, 10.0.0.1"},
			trusted:    trustedProxies,
			want:       "10.0.0.1",
		},
		{
			name:       "CIDR entry is malformed",
			remoteAddr: "10.0.0.1:443",
			xff:        []string{"203.0.113.0/24"},
			trusted:    trustedProxies,
			want:       "10.0.0.1",
		},
		{
			name:       "malformed entry left of the client is never reached",
			remoteAddr: "10.0.0.1:443",
			xff:        []string{"garbage, 203.0.113.7"},
			trusted:    trustedProxies,
			want:       "203.0.113.7",
		},

		// IPv4-mapped IPv6 addresses behave as their IPv4 form.
		{
			name:       "IPv4-mapped RemoteAddr is unmapped",
			remoteAddr: "[::ffff:192.0.2.1]:54321",
			want:       "192.0.2.1",
		},
		{
			name:       "IPv4-mapped peer matches IPv4 trusted prefix",
			remoteAddr: "[::ffff:10.0.0.1]:443",
			xff:        []string{"203.0.113.7"},
			trusted:    trustedProxies,
			want:       "203.0.113.7",
		},
		{
			name:       "IPv4-mapped entries are unmapped and checked",
			remoteAddr: "10.0.0.2:443",
			xff:        []string{"::ffff:203.0.113.7, ::ffff:10.0.0.1"},
			trusted:    trustedProxies,
			want:       "203.0.113.7",
		},
		{
			name:       "IPv4-mapped trusted prefix matches IPv4 peer",
			remoteAddr: "10.0.0.1:443",
			xff:        []string{"203.0.113.7"},
			trusted:    []netip.Prefix{netip.MustParsePrefix("::ffff:10.0.0.0/104")},
			want:       "203.0.113.7",
		},
		{
			name:       "IPv4-mapped trusted prefix skips IPv4 hops in the chain",
			remoteAddr: "10.0.0.2:443",
			xff:        []string{"203.0.113.7, 10.0.0.1"},
			trusted:    []netip.Prefix{netip.MustParsePrefix("::ffff:10.0.0.0/104")},
			want:       "203.0.113.7",
		},
		{
			name:       "mapped prefix under 96 bits trusts IPv4 peer",
			remoteAddr: "10.0.0.1:443",
			xff:        []string{"203.0.113.7"},
			trusted:    []netip.Prefix{netip.MustParsePrefix("::ffff:10.0.0.0/95")},
			want:       "203.0.113.7",
		},
		{
			name:       "IPv6 prefix outside the mapped range ignores IPv4 peer",
			remoteAddr: "10.0.0.1:443",
			xff:        []string{"203.0.113.7"},
			trusted:    []netip.Prefix{netip.MustParsePrefix("::fffe:0:0/96")},
			want:       "10.0.0.1",
		},

		// X-Real-IP is never trusted, even from a trusted proxy.
		{
			name:       "X-Real-IP ignored from untrusted peer",
			remoteAddr: "192.0.2.1:54321",
			realIP:     "203.0.113.7",
			trusted:    trustedProxies,
			want:       "192.0.2.1",
		},
		{
			name:       "X-Real-IP ignored from trusted peer",
			remoteAddr: "10.0.0.1:443",
			realIP:     "203.0.113.7",
			trusted:    trustedProxies,
			want:       "10.0.0.1",
		},
		{
			name:       "X-Forwarded-For wins over X-Real-IP from trusted peer",
			remoteAddr: "10.0.0.1:443",
			xff:        []string{"203.0.113.7"},
			realIP:     "198.51.100.66",
			trusted:    trustedProxies,
			want:       "203.0.113.7",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := newClientIPRequest(tt.remoteAddr, tt.xff, tt.realIP)

			got := ClientIP(req, tt.trusted)

			if want := netip.MustParseAddr(tt.want); got != want {
				t.Errorf("ClientIP() = %v, want %v", got, want)
			}
		})
	}
}

func TestIsTrusted_IPv4MappedPrefixes(t *testing.T) {
	// ClientIP passes normalized addresses, so IPv4 arrives unmapped. A
	// mapped prefix must trust exactly the IPv4 addresses whose mapped form
	// it contains, at any length: below, at and above the 96-bit boundary
	// of ::ffff:0:0/96, and whether or not its host bits are zero.
	tests := []struct {
		prefix string
		addr   string
		want   bool
	}{
		// Above 96 bits: only the IPv4 range the prefix maps.
		{prefix: "::ffff:10.0.0.0/104", addr: "10.0.0.1", want: true},
		{prefix: "::ffff:10.0.0.0/104", addr: "11.0.0.1", want: false},
		{prefix: "::ffff:10.0.0.1/128", addr: "10.0.0.1", want: true},
		{prefix: "::ffff:10.0.0.1/128", addr: "10.0.0.2", want: false},
		{prefix: "::ffff:10.0.0.1/104", addr: "10.255.0.1", want: true},

		// Exactly 96 bits: every IPv4 address and no other IPv6 address.
		{prefix: "::ffff:0.0.0.0/96", addr: "10.0.0.1", want: true},
		{prefix: "::ffff:0.0.0.0/96", addr: "203.0.113.7", want: true},
		{prefix: "::ffff:0.0.0.0/96", addr: "::1", want: false},
		{prefix: "::ffff:0.0.0.0/96", addr: "::fffe:0:1", want: false},

		// Below 96 bits: the prefix covers all of ::ffff:0:0/96 plus other
		// IPv6 space. ::ffff:10.0.0.0/95 is ::fffe:0:0/95 once masked.
		{prefix: "::ffff:10.0.0.0/95", addr: "10.0.0.1", want: true},
		{prefix: "::ffff:10.0.0.0/95", addr: "203.0.113.7", want: true},
		{prefix: "::ffff:10.0.0.0/95", addr: "::fffe:0:1", want: true},
		{prefix: "::ffff:10.0.0.0/95", addr: "::1", want: false},
		{prefix: "::ffff:10.0.0.0/80", addr: "203.0.113.7", want: true},
		{prefix: "::ffff:10.0.0.0/80", addr: "::1", want: true},
		{prefix: "::ffff:10.0.0.0/80", addr: "2001:db8::1", want: false},

		// The same set written without a mapped address behaves the same.
		{prefix: "::/80", addr: "203.0.113.7", want: true},
		{prefix: "::/0", addr: "203.0.113.7", want: true},

		// IPv6 prefixes that do not reach ::ffff:0:0/96 trust no IPv4.
		{prefix: "::fffe:0:0/96", addr: "10.0.0.1", want: false},
		{prefix: "::1/128", addr: "10.0.0.1", want: false},
		{prefix: "2001:db8:ffff::/48", addr: "10.0.0.1", want: false},

		// Plain prefixes keep their usual meaning.
		{prefix: "10.0.0.0/8", addr: "10.0.0.1", want: true},
		{prefix: "10.0.0.0/8", addr: "11.0.0.1", want: false},
		{prefix: "0.0.0.0/0", addr: "2001:db8::1", want: false},
		{prefix: "2001:db8:ffff::/48", addr: "2001:db8:ffff::1", want: true},
	}

	for _, tt := range tests {
		t.Run(tt.prefix+" "+tt.addr, func(t *testing.T) {
			trusted := []netip.Prefix{netip.MustParsePrefix(tt.prefix)}

			if got := isTrusted(netip.MustParseAddr(tt.addr), trusted); got != tt.want {
				t.Errorf("isTrusted(%s, [%s]) = %v, want %v", tt.addr, tt.prefix, got, tt.want)
			}
		})
	}
}

func TestIsTrusted_ZeroPrefixTrustsNothing(t *testing.T) {
	trusted := []netip.Prefix{{}}

	for _, addr := range []string{"10.0.0.1", "::1"} {
		if isTrusted(netip.MustParseAddr(addr), trusted) {
			t.Errorf("isTrusted(%s, [zero Prefix]) = true, want false", addr)
		}
	}
}

func TestClientIP_InvalidRemoteAddrReturnsZeroAddr(t *testing.T) {
	// Every address is trusted here, so a fallback to the headers would be
	// visible: an invalid peer must still yield the zero Addr.
	trustAll := []netip.Prefix{
		netip.MustParsePrefix("0.0.0.0/0"),
		netip.MustParsePrefix("::/0"),
	}

	remoteAddrs := []string{
		"",
		"garbage",
		"192.0.2.1:",
		"192.0.2.1:port",
		"[2001:db8::1",
		"256.0.0.1:54321",
		"@",
	}

	for _, remoteAddr := range remoteAddrs {
		t.Run(remoteAddr, func(t *testing.T) {
			req := newClientIPRequest(remoteAddr, []string{"203.0.113.7"}, "198.51.100.66")

			got := ClientIP(req, trustAll)

			if got.IsValid() {
				t.Errorf("ClientIP() = %v, want the zero netip.Addr", got)
			}
		})
	}
}
