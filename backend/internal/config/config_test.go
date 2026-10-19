package config

import (
	"net/netip"
	"os"
	"reflect"
	"strconv"
	"testing"
	"time"

	"github.com/openai/openai-go/v3/shared"
)

// setRequiredEnv sets the variables Load requires, so each test only has to
// set the variable it is about. t.Setenv restores them after the test.
func setRequiredEnv(t *testing.T) {
	t.Helper()

	t.Setenv("DATABASE_URL", "postgres://test:test@localhost:5432/test")
	t.Setenv("OPENAI_API_KEY", "test-key")
}

func TestLoad_CORSAllowedOrigin(t *testing.T) {
	tests := []struct {
		name  string
		unset bool
		value string
		want  string
	}{
		{name: "unset uses default", unset: true, want: "http://localhost:3000"},
		{name: "empty uses default", value: "", want: "http://localhost:3000"},
		{name: "custom origin", value: "https://app.tickordo.example", want: "https://app.tickordo.example"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			setRequiredEnv(t)

			// t.Setenv first so the original value is restored after the test,
			// even when the variable is then removed.
			t.Setenv("CORS_ALLOWED_ORIGIN", tt.value)
			if tt.unset {
				os.Unsetenv("CORS_ALLOWED_ORIGIN")
			}

			cfg, err := Load()
			if err != nil {
				t.Fatalf("Load() error = %v", err)
			}

			if cfg.CORSAllowedOrigin != tt.want {
				t.Errorf("CORSAllowedOrigin = %q, want %q", cfg.CORSAllowedOrigin, tt.want)
			}
		})
	}
}

// unsetEnv removes key for the duration of the test. t.Setenv runs first so
// the original value is restored afterwards.
func unsetEnv(t *testing.T, key string) {
	t.Helper()

	t.Setenv(key, "")
	os.Unsetenv(key)
}

func TestLoad_AllVariablesSet(t *testing.T) {
	t.Setenv("PORT", "9090")
	t.Setenv("DATABASE_URL", "postgres://test:test@localhost:5432/test")
	t.Setenv("OPENAI_API_KEY", "test-key")
	t.Setenv("OPENAI_MODEL", "gpt-test-model")
	t.Setenv("OPENAI_REASONING_EFFORT", "low")
	t.Setenv("CORS_ALLOWED_ORIGIN", "https://app.tickordo.example")
	t.Setenv("RATE_LIMIT_TRIAGE_REQUESTS", "7")
	t.Setenv("RATE_LIMIT_TRIAGE_WINDOW", "15m")
	t.Setenv("RATE_LIMIT_TICKETS_REQUESTS", "30")
	t.Setenv("RATE_LIMIT_TICKETS_WINDOW", "1h")
	t.Setenv("TRIAGE_GLOBAL_LIMIT", "250")
	t.Setenv("TRIAGE_GLOBAL_WINDOW", "12h")
	t.Setenv("TRIAGE_MAX_CONCURRENT", "2")
	t.Setenv("TRUSTED_PROXY_CIDRS", "10.0.0.0/8, 2001:db8::/32")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	want := Config{
		Port:              "9090",
		DatabaseURL:       "postgres://test:test@localhost:5432/test",
		OpenAIAPIKey:      "test-key",
		OpenAIModel:       "gpt-test-model",
		CORSAllowedOrigin: "https://app.tickordo.example",

		OpenAIReasoningEffort: shared.ReasoningEffortLow,

		TriageRateLimit:    7,
		TriageRateWindow:    15 * time.Minute,
		TicketsRateLimit:    30,
		TicketsRateWindow:   time.Hour,
		TriageGlobalLimit:   250,
		TriageGlobalWindow:  12 * time.Hour,
		TriageMaxConcurrent: 2,
		TrustedProxies: []netip.Prefix{
			netip.MustParsePrefix("10.0.0.0/8"),
			netip.MustParsePrefix("2001:db8::/32"),
		},
	}

	if !reflect.DeepEqual(cfg, want) {
		t.Errorf("Load() = %+v, want %+v", cfg, want)
	}
}

func TestLoad_Port(t *testing.T) {
	tests := []struct {
		name  string
		unset bool
		value string
		want  string
	}{
		{name: "unset uses default", unset: true, want: "8080"},
		{name: "empty uses default", value: "", want: "8080"},
		{name: "custom port", value: "9090", want: "9090"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			setRequiredEnv(t)

			t.Setenv("PORT", tt.value)
			if tt.unset {
				os.Unsetenv("PORT")
			}

			cfg, err := Load()
			if err != nil {
				t.Fatalf("Load() error = %v", err)
			}

			if cfg.Port != tt.want {
				t.Errorf("Port = %q, want %q", cfg.Port, tt.want)
			}
		})
	}
}

func TestLoad_OpenAIModel(t *testing.T) {
	tests := []struct {
		name  string
		unset bool
		value string
		want  string
	}{
		{name: "unset uses default", unset: true, want: "gpt-5.6"},
		{name: "empty uses default", value: "", want: "gpt-5.6"},
		{name: "custom model", value: "gpt-test-model", want: "gpt-test-model"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			setRequiredEnv(t)

			t.Setenv("OPENAI_MODEL", tt.value)
			if tt.unset {
				os.Unsetenv("OPENAI_MODEL")
			}

			cfg, err := Load()
			if err != nil {
				t.Fatalf("Load() error = %v", err)
			}

			if cfg.OpenAIModel != tt.want {
				t.Errorf("OpenAIModel = %q, want %q", cfg.OpenAIModel, tt.want)
			}
		})
	}
}

func TestLoad_OpenAIReasoningEffort(t *testing.T) {
	tests := []struct {
		name  string
		unset bool
		value string
		want  shared.ReasoningEffort
	}{
		{name: "unset omits effort", unset: true, want: ""},
		{name: "empty omits effort", value: "", want: ""},
		{name: "low", value: "low", want: shared.ReasoningEffortLow},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			setRequiredEnv(t)

			t.Setenv("OPENAI_REASONING_EFFORT", tt.value)
			if tt.unset {
				os.Unsetenv("OPENAI_REASONING_EFFORT")
			}

			cfg, err := Load()
			if err != nil {
				t.Fatalf("Load() error = %v", err)
			}

			if cfg.OpenAIReasoningEffort != tt.want {
				t.Errorf("OpenAIReasoningEffort = %q, want %q", cfg.OpenAIReasoningEffort, tt.want)
			}
		})
	}
}

func TestLoad_InvalidOpenAIReasoningEffort(t *testing.T) {
	// Values must match the SDK constants exactly, like the other variables.
	invalid := []string{"turbo", "LOW", " low", "low "}

	for _, value := range invalid {
		t.Run(strconv.Quote(value), func(t *testing.T) {
			setRequiredEnv(t)
			t.Setenv("OPENAI_REASONING_EFFORT", value)

			cfg, err := Load()

			want := "invalid OPENAI_REASONING_EFFORT: must be one of none, minimal, low, medium, high, xhigh, max, got " + strconv.Quote(value)
			if err == nil || err.Error() != want {
				t.Fatalf("Load() error = %v, want %q", err, want)
			}

			if !reflect.DeepEqual(cfg, Config{}) {
				t.Errorf("Load() config = %+v, want zero value", cfg)
			}
		})
	}
}

func TestLoad_MissingRequiredVariable(t *testing.T) {
	tests := []struct {
		name    string
		key     string
		unset   bool
		wantErr string
	}{
		{name: "DATABASE_URL unset", key: "DATABASE_URL", unset: true, wantErr: "missing required environment variable: DATABASE_URL"},
		{name: "DATABASE_URL empty", key: "DATABASE_URL", wantErr: "missing required environment variable: DATABASE_URL"},
		{name: "OPENAI_API_KEY unset", key: "OPENAI_API_KEY", unset: true, wantErr: "missing required environment variable: OPENAI_API_KEY"},
		{name: "OPENAI_API_KEY empty", key: "OPENAI_API_KEY", wantErr: "missing required environment variable: OPENAI_API_KEY"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			setRequiredEnv(t)

			if tt.unset {
				unsetEnv(t, tt.key)
			} else {
				t.Setenv(tt.key, "")
			}

			cfg, err := Load()
			if err == nil {
				t.Fatalf("Load() error = nil, want %q", tt.wantErr)
			}

			if err.Error() != tt.wantErr {
				t.Errorf("Load() error = %q, want %q", err.Error(), tt.wantErr)
			}

			// A failed load must not return a partially filled config.
			if !reflect.DeepEqual(cfg, Config{}) {
				t.Errorf("Load() config = %+v, want zero value", cfg)
			}
		})
	}
}

func TestLoad_ChecksDatabaseURLFirst(t *testing.T) {
	// With both required variables missing, the error names DATABASE_URL.
	unsetEnv(t, "DATABASE_URL")
	unsetEnv(t, "OPENAI_API_KEY")

	_, err := Load()

	want := "missing required environment variable: DATABASE_URL"
	if err == nil || err.Error() != want {
		t.Errorf("Load() error = %v, want %q", err, want)
	}
}

// limitKeys lists the optional limit variables, so tests can clear them and
// start from the defaults.
var limitKeys = []string{
	"RATE_LIMIT_TRIAGE_REQUESTS",
	"RATE_LIMIT_TRIAGE_WINDOW",
	"RATE_LIMIT_TICKETS_REQUESTS",
	"RATE_LIMIT_TICKETS_WINDOW",
	"TRIAGE_GLOBAL_LIMIT",
	"TRIAGE_GLOBAL_WINDOW",
	"TRIAGE_MAX_CONCURRENT",
	"TRUSTED_PROXY_CIDRS",
}

func TestLoad_LimitDefaults(t *testing.T) {
	setRequiredEnv(t)

	for _, key := range limitKeys {
		unsetEnv(t, key)
	}

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.TriageRateLimit != 5 {
		t.Errorf("TriageRateLimit = %d, want 5", cfg.TriageRateLimit)
	}
	if cfg.TriageRateWindow != 10*time.Minute {
		t.Errorf("TriageRateWindow = %v, want 10m", cfg.TriageRateWindow)
	}
	if cfg.TicketsRateLimit != 20 {
		t.Errorf("TicketsRateLimit = %d, want 20", cfg.TicketsRateLimit)
	}
	if cfg.TicketsRateWindow != 10*time.Minute {
		t.Errorf("TicketsRateWindow = %v, want 10m", cfg.TicketsRateWindow)
	}
	if cfg.TriageGlobalLimit != 100 {
		t.Errorf("TriageGlobalLimit = %d, want 100", cfg.TriageGlobalLimit)
	}
	if cfg.TriageGlobalWindow != 24*time.Hour {
		t.Errorf("TriageGlobalWindow = %v, want 24h", cfg.TriageGlobalWindow)
	}
	if cfg.TriageMaxConcurrent != 3 {
		t.Errorf("TriageMaxConcurrent = %d, want 3", cfg.TriageMaxConcurrent)
	}
	if len(cfg.TrustedProxies) != 0 {
		t.Errorf("TrustedProxies = %v, want empty", cfg.TrustedProxies)
	}
}

func TestLoad_PositiveIntegers(t *testing.T) {
	keys := []struct {
		key      string
		fallback int
		get      func(Config) int
	}{
		{key: "RATE_LIMIT_TRIAGE_REQUESTS", fallback: 5, get: func(c Config) int { return c.TriageRateLimit }},
		{key: "RATE_LIMIT_TICKETS_REQUESTS", fallback: 20, get: func(c Config) int { return c.TicketsRateLimit }},
		{key: "TRIAGE_GLOBAL_LIMIT", fallback: 100, get: func(c Config) int { return c.TriageGlobalLimit }},
		{key: "TRIAGE_MAX_CONCURRENT", fallback: 3, get: func(c Config) int { return c.TriageMaxConcurrent }},
	}

	for _, k := range keys {
		t.Run(k.key, func(t *testing.T) {
			valid := []struct {
				value string
				want  int
			}{
				{value: "", want: k.fallback},
				{value: "1", want: 1},
				{value: "42", want: 42},
			}

			for _, v := range valid {
				t.Run("valid "+strconv.Quote(v.value), func(t *testing.T) {
					setRequiredEnv(t)
					t.Setenv(k.key, v.value)

					cfg, err := Load()
					if err != nil {
						t.Fatalf("Load() error = %v", err)
					}

					if got := k.get(cfg); got != v.want {
						t.Errorf("%s = %d, want %d", k.key, got, v.want)
					}
				})
			}

			invalid := []string{"0", "-1", "abc", "1.5", " 5", "5 ", "99999999999999999999"}

			for _, value := range invalid {
				t.Run("invalid "+strconv.Quote(value), func(t *testing.T) {
					setRequiredEnv(t)
					t.Setenv(k.key, value)

					cfg, err := Load()

					want := "invalid " + k.key + ": must be a positive integer, got " + strconv.Quote(value)
					if err == nil || err.Error() != want {
						t.Fatalf("Load() error = %v, want %q", err, want)
					}

					if !reflect.DeepEqual(cfg, Config{}) {
						t.Errorf("Load() config = %+v, want zero value", cfg)
					}
				})
			}
		})
	}
}

func TestLoad_PositiveDurations(t *testing.T) {
	keys := []struct {
		key      string
		fallback time.Duration
		get      func(Config) time.Duration
	}{
		{key: "RATE_LIMIT_TRIAGE_WINDOW", fallback: 10 * time.Minute, get: func(c Config) time.Duration { return c.TriageRateWindow }},
		{key: "RATE_LIMIT_TICKETS_WINDOW", fallback: 10 * time.Minute, get: func(c Config) time.Duration { return c.TicketsRateWindow }},
		{key: "TRIAGE_GLOBAL_WINDOW", fallback: 24 * time.Hour, get: func(c Config) time.Duration { return c.TriageGlobalWindow }},
	}

	for _, k := range keys {
		t.Run(k.key, func(t *testing.T) {
			valid := []struct {
				value string
				want  time.Duration
			}{
				{value: "", want: k.fallback},
				{value: "30s", want: 30 * time.Second},
				{value: "1h30m", want: 90 * time.Minute},
			}

			for _, v := range valid {
				t.Run("valid "+strconv.Quote(v.value), func(t *testing.T) {
					setRequiredEnv(t)
					t.Setenv(k.key, v.value)

					cfg, err := Load()
					if err != nil {
						t.Fatalf("Load() error = %v", err)
					}

					if got := k.get(cfg); got != v.want {
						t.Errorf("%s = %v, want %v", k.key, got, v.want)
					}
				})
			}

			// "10" has no unit, which time.ParseDuration rejects.
			invalid := []string{"0", "0s", "-1m", "10", "abc", " 10m"}

			for _, value := range invalid {
				t.Run("invalid "+strconv.Quote(value), func(t *testing.T) {
					setRequiredEnv(t)
					t.Setenv(k.key, value)

					cfg, err := Load()

					want := "invalid " + k.key + `: must be a positive duration such as "10m", got ` + strconv.Quote(value)
					if err == nil || err.Error() != want {
						t.Fatalf("Load() error = %v, want %q", err, want)
					}

					if !reflect.DeepEqual(cfg, Config{}) {
						t.Errorf("Load() config = %+v, want zero value", cfg)
					}
				})
			}
		})
	}
}

func TestLoad_TrustedProxyCIDRs(t *testing.T) {
	tests := []struct {
		name  string
		unset bool
		value string
		want  []string
	}{
		{name: "unset", unset: true},
		{name: "empty", value: ""},
		{name: "blank", value: "  "},
		{name: "single IPv4", value: "10.0.0.0/8", want: []string{"10.0.0.0/8"}},
		{name: "single IPv4 host", value: "192.0.2.10/32", want: []string{"192.0.2.10/32"}},
		{name: "single IPv6", value: "2001:db8::/32", want: []string{"2001:db8::/32"}},
		{name: "single IPv6 host", value: "2001:db8::1/128", want: []string{"2001:db8::1/128"}},
		{name: "IPv4-mapped /8", value: "::ffff:10.0.0.0/104", want: []string{"::ffff:10.0.0.0/104"}},
		{
			name:  "mixed list with spaces",
			value: " 10.0.0.0/8 ,172.16.0.0/12, 2001:db8:1::/48 ",
			want:  []string{"10.0.0.0/8", "172.16.0.0/12", "2001:db8:1::/48"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			setRequiredEnv(t)

			t.Setenv("TRUSTED_PROXY_CIDRS", tt.value)
			if tt.unset {
				os.Unsetenv("TRUSTED_PROXY_CIDRS")
			}

			cfg, err := Load()
			if err != nil {
				t.Fatalf("Load() error = %v", err)
			}

			var want []netip.Prefix
			for _, s := range tt.want {
				want = append(want, netip.MustParsePrefix(s))
			}

			if !reflect.DeepEqual(cfg.TrustedProxies, want) {
				t.Errorf("TrustedProxies = %v, want %v", cfg.TrustedProxies, want)
			}
		})
	}
}

func TestLoad_InvalidTrustedProxyCIDRs(t *testing.T) {
	const notCIDR = "must be a CIDR such as 10.0.0.0/8 or 2001:db8::/32"

	tests := []struct {
		name    string
		value   string
		wantErr string
	}{
		{name: "not a CIDR", value: "proxy.internal", wantErr: `entry "proxy.internal": ` + notCIDR},
		{name: "address without prefix length", value: "10.0.0.1", wantErr: `entry "10.0.0.1": ` + notCIDR},
		{name: "IPv4 prefix length out of range", value: "10.0.0.0/33", wantErr: `entry "10.0.0.0/33": ` + notCIDR},
		{name: "IPv6 with zone", value: "fe80::%eth0/64", wantErr: `entry "fe80::%eth0/64": ` + notCIDR},
		{name: "IPv4 host bits set", value: "10.0.0.1/8", wantErr: `entry "10.0.0.1/8": host bits are set, use 10.0.0.0/8`},
		{name: "IPv6 host bits set", value: "2001:db8::1/32", wantErr: `entry "2001:db8::1/32": host bits are set, use 2001:db8::/32`},
		{name: "empty entry between commas", value: "10.0.0.0/8,,192.168.0.0/16", wantErr: `entry "": empty entry`},
		{name: "trailing comma", value: "10.0.0.0/8,", wantErr: `entry "": empty entry`},
		{name: "invalid entry after valid one", value: "10.0.0.0/8, nope", wantErr: `entry "nope": ` + notCIDR},

		// Prefixes broader than a proxy network are rejected.
		{name: "every IPv4 address", value: "0.0.0.0/0", wantErr: `entry "0.0.0.0/0": IPv4 prefix is broader than /8`},
		{name: "IPv4 broader than /8", value: "10.0.0.0/7", wantErr: `entry "10.0.0.0/7": IPv4 prefix is broader than /8`},
		{name: "every IPv6 address", value: "::/0", wantErr: `entry "::/0": IPv6 prefix is broader than /32`},
		{name: "IPv6 broader than /32", value: "2000::/3", wantErr: `entry "2000::/3": IPv6 prefix is broader than /32`},
		{name: "IPv6 covering the IPv4-mapped range", value: "::/80", wantErr: `entry "::/80": prefix covers every IPv4-mapped address`},
		{name: "whole IPv4-mapped range", value: "::ffff:0:0/96", wantErr: `entry "::ffff:0:0/96": prefix covers every IPv4-mapped address`},
		{name: "IPv4-mapped broader than /8", value: "::ffff:0.0.0.0/100", wantErr: `entry "::ffff:0.0.0.0/100": IPv4-mapped prefix is broader than /8`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			setRequiredEnv(t)
			t.Setenv("TRUSTED_PROXY_CIDRS", tt.value)

			cfg, err := Load()

			want := "invalid TRUSTED_PROXY_CIDRS " + tt.wantErr
			if err == nil || err.Error() != want {
				t.Fatalf("Load() error = %v, want %q", err, want)
			}

			if !reflect.DeepEqual(cfg, Config{}) {
				t.Errorf("Load() config = %+v, want zero value", cfg)
			}
		})
	}
}
