package render

// [>] 🤖🤖

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"time"

	secretmanager "cloud.google.com/go/secretmanager/apiv1"
	secretmanagerpb "cloud.google.com/go/secretmanager/apiv1/secretmanagerpb"

	"gitlab.com/konradodwrot/go-modules/che/internal/execx"
)

const opBin = "op"

var secretSchemes = []string{"op://", "gcp://"}

var secretRetryDelays = []time.Duration{
	500 * time.Millisecond,
	1 * time.Second,
	2 * time.Second,
}

var secretSleep = time.Sleep

func newSecretFunc(ctx context.Context) func(string) (string, error) {
	cache := map[string]secretResolver{}
	return func(ref string) (string, error) {
		scheme := schemeOf(ref)
		factory := lookupFactory(scheme)
		if factory == nil {
			return "", fmt.Errorf("secret %q: unknown scheme (want %s)", ref, strings.Join(secretSchemes, ", "))
		}
		backend := cache[scheme]
		if backend == nil {
			b, err := factory(ctx)
			if err != nil {
				return "", fmt.Errorf("secret %q: %w", ref, err)
			}
			backend = b
			cache[scheme] = b
		}
		secret, err := retry(secretRetryDelays, secretSleep, isRateLimitErr, func() (string, error) {
			return backend.Resolve(ctx, ref)
		})
		if err != nil {
			return "", fmt.Errorf("secret resolve %q: %w", ref, err)
		}
		return secret, nil
	}
}

func lookupFactory(scheme string) func(context.Context) (secretResolver, error) {
	switch scheme {
	case "op://":
		return newOpBackend
	case "gcp://":
		return newGCPBackend
	}
	return nil
}

type secretResolver interface {
	Resolve(ctx context.Context, ref string) (string, error)
}

var newOpBackend = func(_ context.Context) (secretResolver, error) {
	if os.Getenv("OP_SERVICE_ACCOUNT_TOKEN") == "" {
		return nil, fmt.Errorf("OP_SERVICE_ACCOUNT_TOKEN unset")
	}
	if _, err := exec.LookPath(opBin); err != nil {
		return nil, fmt.Errorf("%s not found in PATH: %w", opBin, err)
	}
	return opBackend{}, nil
}

func (opBackend) Resolve(ctx context.Context, ref string) (string, error) {
	var stderr bytes.Buffer
	out, err := execx.Default.Output(execx.Cmd{
		Ctx:    ctx,
		Argv:   []string{opBin, "read", "--no-newline", ref},
		Stderr: &stderr,
	})
	if err != nil {
		if msg := strings.TrimSpace(stderr.String()); msg != "" {
			return "", fmt.Errorf("%s read: %s: %w", opBin, msg, err)
		}
		return "", fmt.Errorf("%s read: %w", opBin, err)
	}
	return strings.TrimRight(string(out), "\n"), nil
}

var newGCPBackend = func(ctx context.Context) (secretResolver, error) {
	client, err := secretmanager.NewClient(ctx)
	if err != nil {
		return nil, err
	}
	return gcpBackend{client}, nil
}

func (r gcpBackend) Resolve(ctx context.Context, ref string) (string, error) {
	project, secret, version, err := parseGCPRef(ref)
	if err != nil {
		return "", err
	}
	name := fmt.Sprintf("projects/%s/secrets/%s/versions/%s", project, secret, version)
	resp, err := r.client.AccessSecretVersion(ctx, &secretmanagerpb.AccessSecretVersionRequest{Name: name})
	if err != nil {
		return "", err
	}
	return string(resp.GetPayload().GetData()), nil
}

func parseGCPRef(ref string) (project, secret, version string, err error) {
	rest := strings.TrimPrefix(ref, "gcp://")
	parts := strings.Split(rest, "/")
	if len(parts) < 2 || parts[0] == "" || parts[1] == "" {
		return "", "", "", fmt.Errorf("malformed gcp ref %q: want gcp://<project>/<secret>[/<version>]", ref)
	}
	version = "latest"
	if len(parts) >= 3 && parts[2] != "" {
		version = parts[2]
	}
	return parts[0], parts[1], version, nil
}

func retry[T any](delays []time.Duration, sleep func(time.Duration), shouldRetry func(error) bool, op func() (T, error)) (T, error) {
	out, err := op()
	for _, delay := range delays {
		if !shouldRetry(err) {
			break
		}
		sleep(delay)
		out, err = op()
	}
	return out, err
}

func isRateLimitErr(err error) bool {
	return err != nil && strings.Contains(err.Error(), "rate limit exceeded")
}

var secretActionPattern = regexp.MustCompile(`\{\{[^}]*?(op|gcp):\/\/`)

// IsSecretRefPresent reports whether a template body resolves an op:// or gcp:// ref inside an action.
func IsSecretRefPresent(body []byte) bool {
	return secretActionPattern.Match(body)
}

func schemeOf(ref string) string {
	for _, s := range secretSchemes {
		if strings.HasPrefix(ref, s) {
			return s
		}
	}
	return ""
}

//[<] 🤖🤖
