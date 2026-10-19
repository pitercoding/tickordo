package config

import (
	"fmt"
	"net/netip"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/openai/openai-go/v3/shared"
)

type Config struct {
	Port              string
	DatabaseURL       string
	OpenAIAPIKey      string
	OpenAIModel       string
	CORSAllowedOrigin string

	// Reasoning effort sent to OpenAI. Empty means the field is omitted and
	// the model uses its own default.
	OpenAIReasoningEffort shared.ReasoningEffort

	// Per-client limit on triage requests.
	TriageRateLimit  int
	TriageRateWindow time.Duration

	// Per-client limit on ticket creation.
	TicketsRateLimit  int
	TicketsRateWindow time.Duration

	// Limit on triages across all clients, which bounds OpenAI usage.
	TriageGlobalLimit  int
	TriageGlobalWindow time.Duration

	// Maximum number of triages running at the same time.
	TriageMaxConcurrent int

	// Proxies whose X-Forwarded-For entries are trusted when resolving the
	// client IP. Empty means only RemoteAddr is used.
	TrustedProxies []netip.Prefix
}

const (
	// minIPv4ProxyBits and minIPv6ProxyBits are the shortest prefixes
	// accepted in TRUSTED_PROXY_CIDRS. Anything broader, such as 0.0.0.0/0
	// or ::/0, would let arbitrary clients choose their own IP.
	minIPv4ProxyBits = 8
	minIPv6ProxyBits = 32
)

// reasoningEfforts lists the OPENAI_REASONING_EFFORT values the SDK defines.
// A model may still reject some of them; the SDK does not check that.
var reasoningEfforts = []shared.ReasoningEffort{
	shared.ReasoningEffortNone,
	shared.ReasoningEffortMinimal,
	shared.ReasoningEffortLow,
	shared.ReasoningEffortMedium,
	shared.ReasoningEffortHigh,
	shared.ReasoningEffortXhigh,
	shared.ReasoningEffortMax,
}

// ipv4MappedRange holds the IPv4-mapped IPv6 addresses. ClientIP matches IPv4
// clients against prefixes in this range too, so they follow the IPv4 rule.
var ipv4MappedRange = netip.MustParsePrefix("::ffff:0:0/96")

// Load reads the configuration from environment variables and returns an
// error when a required variable is missing or a variable is invalid.
func Load() (Config, error) {
	port := os.Getenv("PORT")

	if port == "" {
		port = "8080"
	}

	// Default to the local frontend so development works without extra setup.
	corsAllowedOrigin := os.Getenv("CORS_ALLOWED_ORIGIN")

	if corsAllowedOrigin == "" {
		corsAllowedOrigin = "http://localhost:3000"
	}

	// Default to the model the analyzer has used so far, so existing
	// deployments keep their behavior when OPENAI_MODEL is not set.
	openAIModel := os.Getenv("OPENAI_MODEL")

	if openAIModel == "" {
		openAIModel = "gpt-5.6"
	}

	cfg := Config{
		Port:              port,
		DatabaseURL:       os.Getenv("DATABASE_URL"),
		OpenAIAPIKey:      os.Getenv("OPENAI_API_KEY"),
		OpenAIModel:       openAIModel,
		CORSAllowedOrigin: corsAllowedOrigin,
	}

	if cfg.DatabaseURL == "" {
		return Config{}, fmt.Errorf("missing required environment variable: DATABASE_URL")
	}

	// Triage depends on OpenAI, so the API key is required at startup.
	if cfg.OpenAIAPIKey == "" {
		return Config{}, fmt.Errorf("missing required environment variable: OPENAI_API_KEY")
	}

	reasoningEffort, err := parseReasoningEffort(os.Getenv("OPENAI_REASONING_EFFORT"))
	if err != nil {
		return Config{}, err
	}

	cfg.OpenAIReasoningEffort = reasoningEffort

	// Limits are optional: unset or empty uses the default, anything else
	// must be valid so a typo fails at startup instead of during a request.
	limits := []struct {
		key      string
		fallback int
		dst      *int
	}{
		{key: "RATE_LIMIT_TRIAGE_REQUESTS", fallback: 5, dst: &cfg.TriageRateLimit},
		{key: "RATE_LIMIT_TICKETS_REQUESTS", fallback: 20, dst: &cfg.TicketsRateLimit},
		{key: "TRIAGE_GLOBAL_LIMIT", fallback: 100, dst: &cfg.TriageGlobalLimit},
		{key: "TRIAGE_MAX_CONCURRENT", fallback: 3, dst: &cfg.TriageMaxConcurrent},
	}

	for _, l := range limits {
		n, err := positiveIntEnv(l.key, l.fallback)
		if err != nil {
			return Config{}, err
		}

		*l.dst = n
	}

	windows := []struct {
		key      string
		fallback time.Duration
		dst      *time.Duration
	}{
		{key: "RATE_LIMIT_TRIAGE_WINDOW", fallback: 10 * time.Minute, dst: &cfg.TriageRateWindow},
		{key: "RATE_LIMIT_TICKETS_WINDOW", fallback: 10 * time.Minute, dst: &cfg.TicketsRateWindow},
		{key: "TRIAGE_GLOBAL_WINDOW", fallback: 24 * time.Hour, dst: &cfg.TriageGlobalWindow},
	}

	for _, w := range windows {
		d, err := positiveDurationEnv(w.key, w.fallback)
		if err != nil {
			return Config{}, err
		}

		*w.dst = d
	}

	trustedProxies, err := parseTrustedProxies(os.Getenv("TRUSTED_PROXY_CIDRS"))
	if err != nil {
		return Config{}, err
	}

	cfg.TrustedProxies = trustedProxies

	return cfg, nil
}

// parseReasoningEffort returns the effort in value, or "" when value is empty
// so the request keeps the model default. Unknown values fail at startup
// instead of on every triage.
func parseReasoningEffort(value string) (shared.ReasoningEffort, error) {
	if value == "" {
		return "", nil
	}

	names := make([]string, len(reasoningEfforts))

	for i, effort := range reasoningEfforts {
		if value == string(effort) {
			return effort, nil
		}

		names[i] = string(effort)
	}

	return "", fmt.Errorf(
		"invalid OPENAI_REASONING_EFFORT: must be one of %s, got %q",
		strings.Join(names, ", "),
		value,
	)
}

// positiveIntEnv returns the positive integer in key, or fallback when key is
// unset or empty.
func positiveIntEnv(key string, fallback int) (int, error) {
	value := os.Getenv(key)
	if value == "" {
		return fallback, nil
	}

	n, err := strconv.Atoi(value)
	if err != nil || n <= 0 {
		return 0, fmt.Errorf("invalid %s: must be a positive integer, got %q", key, value)
	}

	return n, nil
}

// positiveDurationEnv returns the positive duration in key (e.g. "10m"), or
// fallback when key is unset or empty.
func positiveDurationEnv(key string, fallback time.Duration) (time.Duration, error) {
	value := os.Getenv(key)
	if value == "" {
		return fallback, nil
	}

	d, err := time.ParseDuration(value)
	if err != nil || d <= 0 {
		return 0, fmt.Errorf("invalid %s: must be a positive duration such as \"10m\", got %q", key, value)
	}

	return d, nil
}

// parseTrustedProxies parses a comma-separated list of CIDRs. An empty or
// blank value yields no trusted proxies.
func parseTrustedProxies(value string) ([]netip.Prefix, error) {
	if strings.TrimSpace(value) == "" {
		return nil, nil
	}

	var prefixes []netip.Prefix

	for _, entry := range strings.Split(value, ",") {
		entry = strings.TrimSpace(entry)

		prefix, err := parseTrustedProxy(entry)
		if err != nil {
			return nil, fmt.Errorf("invalid TRUSTED_PROXY_CIDRS entry %q: %w", entry, err)
		}

		prefixes = append(prefixes, prefix)
	}

	return prefixes, nil
}

// parseTrustedProxy parses one CIDR and rejects forms that are likely
// mistakes or that would trust far more than a proxy network.
func parseTrustedProxy(entry string) (netip.Prefix, error) {
	if entry == "" {
		return netip.Prefix{}, fmt.Errorf("empty entry")
	}

	prefix, err := netip.ParsePrefix(entry)
	if err != nil {
		return netip.Prefix{}, fmt.Errorf("must be a CIDR such as 10.0.0.0/8 or 2001:db8::/32")
	}

	// 10.0.0.1/8 is ambiguous: a single host or the whole network.
	if masked := prefix.Masked(); masked != prefix {
		return netip.Prefix{}, fmt.Errorf("host bits are set, use %s", masked)
	}

	bits := prefix.Bits()

	switch {
	case prefix.Addr().Is4():
		if bits < minIPv4ProxyBits {
			return netip.Prefix{}, fmt.Errorf("IPv4 prefix is broader than /%d", minIPv4ProxyBits)
		}
	case bits < minIPv6ProxyBits:
		return netip.Prefix{}, fmt.Errorf("IPv6 prefix is broader than /%d", minIPv6ProxyBits)
	case prefix.Contains(ipv4MappedRange.Addr()) && bits <= ipv4MappedRange.Bits():
		// E.g. ::/80 or ::ffff:0:0/96, which ClientIP matches against every
		// IPv4 client.
		return netip.Prefix{}, fmt.Errorf("prefix covers every IPv4-mapped address")
	case ipv4MappedRange.Contains(prefix.Addr()) && bits-ipv4MappedRange.Bits() < minIPv4ProxyBits:
		return netip.Prefix{}, fmt.Errorf("IPv4-mapped prefix is broader than /%d", minIPv4ProxyBits)
	}

	return prefix, nil
}
