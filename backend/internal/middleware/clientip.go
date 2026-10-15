package middleware

import (
	"net/http"
	"net/netip"
	"strings"
)

// ClientIP returns the address of the client that sent r.
//
// By default the client is the peer in r.RemoteAddr. X-Forwarded-For is only
// read when that peer is inside one of the trusted prefixes: the header is then
// walked from right to left, skipping trusted proxies, and the first untrusted
// address is the client. Entries to its left were written by the client itself
// and are never used. X-Real-IP is never read.
//
// If a malformed X-Forwarded-For entry is reached before an untrusted address,
// the walk stops and the last trusted hop is returned, since nothing further
// left can be attributed to a trusted proxy. The same hop is returned when
// every entry is trusted.
//
// The result is normalized: IPv4-mapped IPv6 addresses are unmapped and IPv6
// zones are dropped, so one client always yields the same value. When
// r.RemoteAddr cannot be parsed, the zero netip.Addr is returned (IsValid
// reports false) without looking at any header; callers must handle it.
func ClientIP(r *http.Request, trusted []netip.Prefix) netip.Addr {
	remote, ok := parseAddr(r.RemoteAddr)
	if !ok {
		return netip.Addr{}
	}

	if !isTrusted(remote, trusted) {
		return remote
	}

	// client is the leftmost hop verified so far; it starts at the trusted peer.
	client := remote

	// Each proxy appends to the last header line, so the rightmost entry of
	// the last line is the closest hop.
	lines := r.Header.Values("X-Forwarded-For")

	for i := len(lines) - 1; i >= 0; i-- {
		list := lines[i]

		for {
			comma := strings.LastIndexByte(list, ',')

			addr, ok := parseAddr(strings.TrimSpace(list[comma+1:]))
			if !ok {
				return client
			}

			if !isTrusted(addr, trusted) {
				return addr
			}

			client = addr

			if comma < 0 {
				break
			}

			list = list[:comma]
		}
	}

	return client
}

// parseAddr parses an IP address with or without a port ("192.0.2.1:8080",
// "[2001:db8::1]:443", "192.0.2.1") and normalizes it.
func parseAddr(s string) (netip.Addr, bool) {
	addr, err := netip.ParseAddr(s)
	if err != nil {
		addrPort, err := netip.ParseAddrPort(s)
		if err != nil {
			return netip.Addr{}, false
		}

		addr = addrPort.Addr()
	}

	return addr.Unmap().WithZone(""), true
}

// isTrusted reports whether addr, already normalized, is inside any of the
// trusted prefixes.
//
// Normalization unmaps IPv4-mapped addresses, but Prefix.Contains never
// matches across families. So an IPv4 addr is also checked in its mapped
// form (::ffff:a.b.c.d) against the prefixes as given, without rewriting
// them: ::ffff:10.0.0.0/104 trusts 10.0.0.0/8, a prefix covering all of
// ::ffff:0:0/96 (e.g. ::/80 or ::ffff:10.0.0.0/95) trusts every IPv4
// address, and ::fffe:0:0/96 trusts none.
func isTrusted(addr netip.Addr, trusted []netip.Prefix) bool {
	// The zero Addr when addr is IPv6, which matches no prefix.
	var mapped netip.Addr
	if addr.Is4() {
		mapped = netip.AddrFrom16(addr.As16())
	}

	for _, prefix := range trusted {
		if prefix.Contains(addr) || prefix.Contains(mapped) {
			return true
		}
	}

	return false
}
