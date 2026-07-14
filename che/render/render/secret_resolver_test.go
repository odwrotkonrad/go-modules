package render

// [>] 🤖🤖

import (
	"context"
	"errors"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"

	"gitlab.com/konradodwrot/go-modules/che/internal/testutil"
	"gitlab.com/konradodwrot/go-modules/lib/testyml"
)

// SecretMockResolver is the secretResolver test double: counts Resolve calls,
// failing the first FailLeft with FailMsg, then serving Secret.
type SecretMockResolver struct {
	Calls    int
	FailLeft int
	FailMsg  string
	Secret   string
}

func (m *SecretMockResolver) Resolve(_ context.Context, _ string) (string, error) {
	m.Calls++
	if m.FailLeft > 0 {
		m.FailLeft--
		return "", errors.New(m.FailMsg)
	}
	return m.Secret, nil
}

// secretWant is secret_resolver's expected.output: the resolved value plus the
// per-backend resolver/factory call counts (asserted on error cases too).
type secretWant struct {
	Value           string `yaml:"value"`
	OpCalls         int    `yaml:"opCalls"`
	GcpCalls        int    `yaml:"gcpCalls"`
	OpFactoryCalls  int    `yaml:"opFactoryCalls"`
	GcpFactoryCalls int    `yaml:"gcpFactoryCalls"`
}

// requireSecretResolverMock validates the package-local mockedInterfaces pair.
func requireSecretResolverMock(t *testing.T, decl map[string]string) {
	t.Helper()
	for iface, mock := range decl {
		if iface != "render.secretResolver" || mock != "render.SecretMockResolver" {
			t.Fatalf("mockedInterfaces: unknown pair %s: %s", iface, mock)
		}
	}
}

func TestSecretFunc(t *testing.T) {
	testyml.Run(t, td, "testdata/spec/funcs/secret_resolver.test.spec.yml", func(t *testing.T, c testyml.Case[secretWant]) {
		requireSecretResolverMock(t, c.Context.MockedInterfaces)
		for k, v := range c.Context.Env {
			t.Setenv(k, v)
		}
		a := c.Input.Args
		mr := &SecretMockResolver{FailLeft: a.Int(t, 2), FailMsg: a.String(t, 3), Secret: a.String(t, 1)}
		clientFail := a.Bool(t, 4)

		opFactoryCalls, gcpFactoryCalls := 0, 0
		// op factory mock: replicate the real token gate, then serve the mock.
		testyml.Swap(t, &newOpBackend, func(_ context.Context) (secretResolver, error) {
			opFactoryCalls++
			if os.Getenv("OP_SERVICE_ACCOUNT_TOKEN") == "" {
				return nil, errors.New("OP_SERVICE_ACCOUNT_TOKEN unset")
			}
			if clientFail {
				return nil, errors.New("stub: client init fail")
			}
			return mr, nil
		})
		testyml.Swap(t, &newGCPBackend, func(_ context.Context) (secretResolver, error) {
			gcpFactoryCalls++
			if clientFail {
				return nil, errors.New("stub: client init fail")
			}
			return mr, nil
		})
		testyml.Swap(t, &secretSleep, testutil.SleepMock)

		secret := secretFunc(context.Background())
		ref := a.String(t, 0)
		got, err := secret(ref)
		if a.Bool(t, 5) {
			got, err = secret(ref)
		}
		if !c.Expected.Check(t, err) {
			assert.Equal(t, c.Expected.Output.Value, got, "secret(%q)", ref)
		}
		opCalls, gcpCalls := 0, 0
		switch schemeOf(ref) {
		case "op://":
			opCalls = mr.Calls
		case "gcp://":
			gcpCalls = mr.Calls
		}
		assert.Equal(t, c.Expected.Output.OpCalls, opCalls, "op resolver calls")
		assert.Equal(t, c.Expected.Output.GcpCalls, gcpCalls, "gcp resolver calls")
		assert.Equal(t, c.Expected.Output.OpFactoryCalls, opFactoryCalls, "op factory calls")
		assert.Equal(t, c.Expected.Output.GcpFactoryCalls, gcpFactoryCalls, "gcp factory calls")
	})
}

// [<] 🤖🤖
