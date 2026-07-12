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

// Resolve serves the scenario: fail FailLeft times, then Secret.
func (m *SecretMockResolver) Resolve(_ context.Context, _ string) (string, error) {
	m.Calls++
	if m.FailLeft > 0 {
		m.FailLeft--
		return "", errors.New(m.FailMsg)
	}
	return m.Secret, nil
}

// opWant is op_resolver's expected.output: the resolved value plus the
// resolver/factory call counts (asserted on error cases too).
type opWant struct {
	Value        string `yaml:"value"`
	Calls        int    `yaml:"calls"`
	FactoryCalls int    `yaml:"factoryCalls"`
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

func TestOpResolver(t *testing.T) {
	testyml.Run(t, td, "testdata/spec/op_resolver.test.spec.yml", func(t *testing.T, c testyml.Case[opWant]) {
		requireSecretResolverMock(t, c.Context.MockedInterfaces)
		for k, v := range c.Context.Env {
			t.Setenv(k, v)
		}
		a := c.Input.Args
		mr := &SecretMockResolver{FailLeft: a.Int(t, 2), FailMsg: a.String(t, 3), Secret: a.String(t, 1)}
		factoryCalls := 0
		testyml.Swap(t, &newSecretResolver, func(_ context.Context, token string) (secretResolver, error) {
			factoryCalls++
			if a.Bool(t, 4) {
				return nil, errors.New("stub: client init fail")
			}
			assert.Equal(t, os.Getenv("OP_SERVICE_ACCOUNT_TOKEN"), token, "factory token")
			return mr, nil
		})
		testyml.Swap(t, &opSleep, testutil.SleepMock)
		op := opResolver(context.Background())
		ref := a.String(t, 0)
		got, err := op(ref)
		if a.Bool(t, 5) {
			got, err = op(ref)
		}
		if !c.Expected.Check(t, err) {
			assert.Equal(t, c.Expected.Output.Value, got, "op(%q)", ref)
		}
		assert.Equal(t, c.Expected.Output.Calls, mr.Calls, "resolver calls")
		assert.Equal(t, c.Expected.Output.FactoryCalls, factoryCalls, "factory calls")
	})
}

// [<] 🤖🤖
