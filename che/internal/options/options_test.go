package options

// [>] 🤖🤖

import (
	"testing"

	"gitlab.com/konradodwrot/go-modules/che/internal/spec"
)

func TestResolveBoolOr(t *testing.T) {
	cases := []struct {
		name        string
		env         map[string]string
		user, spec  Layer
		wantEnabled bool
		wantDebug   bool
	}{
		{name: "unset no layers -> off"},
		{name: "env 0 -> off", env: map[string]string{"CHE_OTEL_ENABLED": "0"}},
		{name: "env false -> off", env: map[string]string{"CHE_OTEL_ENABLED": "false"}},
		{name: "env off -> off", env: map[string]string{"CHE_OTEL_ENABLED": "off"}},
		{name: "env no -> off", env: map[string]string{"CHE_OTEL_ENABLED": "no"}},
		{name: "env 1 -> on", env: map[string]string{"CHE_OTEL_ENABLED": "1"}, wantEnabled: true},
		{name: "env true -> on", env: map[string]string{"CHE_OTEL_ENABLED": "true"}, wantEnabled: true},
		{
			name:        "spec layer true -> on",
			spec:        Layer{Otel: spec.Otel{Enabled: new(true)}},
			wantEnabled: true,
		},
		{
			name:        "user false over spec true -> off",
			user:        Layer{Otel: spec.Otel{Enabled: new(false)}},
			spec:        Layer{Otel: spec.Otel{Enabled: new(true)}},
			wantEnabled: false,
		},
		{
			name:        "env 0 beats spec true -> off",
			env:         map[string]string{"CHE_OTEL_ENABLED": "0"},
			spec:        Layer{Otel: spec.Otel{Enabled: new(true)}},
			wantEnabled: false,
		},
		{
			name:      "debug env off -> false",
			env:       map[string]string{"CHE_DEBUG": "off"},
			spec:      Layer{Debug: new(true)},
			wantDebug: false,
		},
		{
			name:      "debug user false over spec true -> false",
			user:      Layer{Debug: new(false)},
			spec:      Layer{Debug: new(true)},
			wantDebug: false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			env := func(k string) string { return tc.env[k] }
			var c Options
			if err := c.Resolve(env, tc.user, tc.spec); err != nil {
				t.Fatalf("Resolve: %v", err)
			}
			if c.Otel.Enabled != tc.wantEnabled {
				t.Errorf("Otel.Enabled = %v, want %v", c.Otel.Enabled, tc.wantEnabled)
			}
			if c.Debug != tc.wantDebug {
				t.Errorf("Debug = %v, want %v", c.Debug, tc.wantDebug)
			}
		})
	}
}

// [<] 🤖🤖
