package e2e

// [>] 🤖🤖

import (
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"gitlab.com/konradodwrot/go-modules/che/internal/packages"
)

// [why] unit tests stub every exec and the real-install e2e samples one package per
//
//	method: only this sweep proves every brew formula and cask name still exists
//	upstream (a renamed cask like ollama -> ollama-app is invisible otherwise)
func TestE2ERegistryBrewNames(t *testing.T) {
	if os.Getenv("E2E_REGISTRY") == "" {
		t.Skip("set E2E_REGISTRY=1 to check upstream registries")
	}
	file, err := packages.LoadBuiltin()
	require.NoError(t, err)
	client := &http.Client{Timeout: 30 * time.Second}
	for name, entry := range file.Packages {
		for _, it := range entry.Items {
			if it.Mgr != "brew" && it.Mgr != "cask" {
				continue
			}
			kind, ref := it.Mgr, it.Name
			if ref == "" {
				ref = name
			}
			pin := entry.Version
			if it.Version != "" {
				pin = it.Version
			}
			if it.Mgr == "brew" && pin != "" && pin != packages.VersionLatest {
				ref += "@" + pin
			}
			if it.Registry != "" {
				ref = it.Registry + "/" + ref
			}
			t.Run(kind+"/"+ref, func(t *testing.T) {
				t.Parallel()
				requireBrewName(t, client, kind, ref)
			})
		}
	}
}

func requireBrewName(t *testing.T, client *http.Client, kind, ref string) {
	t.Helper()
	parts := strings.Split(ref, "/")
	if len(parts) == 1 {
		if kind == "cask" {
			requireAnyURL(t, client, []string{"https://formulae.brew.sh/api/cask/" + ref + ".json"})
			return
		}
		// [why] the api has no per-alias endpoint (kubectl -> kubernetes-cli): the core
		//   repo's Aliases dir answers for names the formula endpoint misses
		requireAnyURL(t, client, []string{
			"https://formulae.brew.sh/api/formula/" + ref + ".json",
			"https://raw.githubusercontent.com/Homebrew/homebrew-core/HEAD/Aliases/" + ref,
		})
		return
	}
	require.Len(t, parts, 3, "tapped name must be user/tap/name: %s", ref)
	user, tap, base := parts[0], parts[1], parts[2]
	var urls []string
	for _, host := range []string{
		"https://raw.githubusercontent.com/" + user + "/homebrew-" + tap + "/HEAD/",
		"https://gitlab.com/" + user + "/homebrew-" + tap + "/-/raw/HEAD/",
	} {
		for _, dir := range []string{"Formula/", "Casks/", "", "Formula/" + base[:1] + "/", "Casks/" + base[:1] + "/"} {
			urls = append(urls, host+dir+base+".rb")
		}
	}
	requireAnyURL(t, client, urls)
}

func requireAnyURL(t *testing.T, client *http.Client, urls []string) {
	t.Helper()
	for _, u := range urls {
		resp, err := client.Get(u)
		if err != nil {
			continue
		}
		_ = resp.Body.Close()
		if resp.StatusCode == http.StatusOK {
			return
		}
	}
	t.Errorf("no upstream source found, tried: %s", strings.Join(urls, ", "))
}

// [<] 🤖🤖
