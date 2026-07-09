package render

// [>] 🤖🤖

import (
	"context"
	"errors"
	"testing"
	"time"

	"gitlab.com/konradodwrot/go-modules/lib/testyml"
)

// mockResolver counts Resolve calls, failing the first failLeft with failMsg.
type mockResolver struct {
	calls    int
	failLeft int
	failMsg  string
	secret   string
}

func (m *mockResolver) Resolve(_ context.Context, _ string) (string, error) {
	m.calls++
	if m.failLeft > 0 {
		m.failLeft--
		return "", errors.New(m.failMsg)
	}
	return m.secret, nil
}

func TestOpResolver(t *testing.T) {
	type in struct {
		Ref          string
		Token        string
		Secret       string
		FailTimes    int    `yaml:"failTimes"`
		FailMsg      string `yaml:"failMsg"`
		ClientFail   bool   `yaml:"clientFail"`
		ResolveTwice bool   `yaml:"resolveTwice"`
	}
	type want struct {
		testyml.Want `yaml:",inline"`
		Value        string
		Calls        int
		FactoryCalls int `yaml:"factoryCalls"`
	}
	type c struct {
		Name string
		In   in
		Want want
	}
	testyml.Run(t, td, "testdata/spec/op_resolver.spec.yml", func(t *testing.T, c c) {
		t.Setenv("OP_SERVICE_ACCOUNT_TOKEN", c.In.Token)
		mr := &mockResolver{failLeft: c.In.FailTimes, failMsg: c.In.FailMsg, secret: c.In.Secret}
		factoryCalls := 0
		prevNew, prevSleep := newSecretResolver, opSleep
		newSecretResolver = func(_ context.Context, token string) (secretResolver, error) {
			factoryCalls++
			if c.In.ClientFail {
				return nil, errors.New("stub: client init fail")
			}
			if token != c.In.Token {
				t.Errorf("factory token = %q, want %q", token, c.In.Token)
			}
			return mr, nil
		}
		opSleep = func(time.Duration) {}
		t.Cleanup(func() { newSecretResolver, opSleep = prevNew, prevSleep })
		op := opResolver(context.Background())
		got, err := op(c.In.Ref)
		if c.In.ResolveTwice {
			got, err = op(c.In.Ref)
		}
		if c.Want.IsErrorWanted() {
			c.Want.CheckErr(t, err)
		} else {
			if err != nil {
				t.Fatalf("op(%q) errored: %v", c.In.Ref, err)
			}
			if got != c.Want.Value {
				t.Errorf("op(%q) = %q, want %q", c.In.Ref, got, c.Want.Value)
			}
		}
		if mr.calls != c.Want.Calls {
			t.Errorf("resolver calls = %d, want %d", mr.calls, c.Want.Calls)
		}
		if factoryCalls != c.Want.FactoryCalls {
			t.Errorf("factory calls = %d, want %d", factoryCalls, c.Want.FactoryCalls)
		}
	})
}

// [<] 🤖🤖
