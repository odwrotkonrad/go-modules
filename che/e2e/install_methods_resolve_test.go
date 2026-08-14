package e2e

// [>] 🤖🤖

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestResolveRunCfg(t *testing.T) {
	cases := []struct {
		name                         string
		platform, mode, hostOS, arch string
		virt, allowHost              bool
		want                         runCfg
		wantErr                      string
	}{
		{
			name: "empty defaults darwin host", hostOS: "darwin", arch: "arm64",
			want: runCfg{mode: "with_no_deps", darwinVM: true},
		},
		{
			name: "empty defaults linux host", hostOS: "linux", arch: "arm64",
			want: runCfg{mode: "with_no_deps"},
		},
		{
			name: "empty with_deps virt", mode: "with_deps", hostOS: "darwin", arch: "arm64", virt: true,
			want: runCfg{mode: "with_deps"},
		},
		{
			name: "empty with_deps bare metal", mode: "with_deps", hostOS: "darwin", arch: "arm64",
			wantErr: "installs never run on the bare-metal host",
		},
		{
			name: "empty with_deps bare metal allowed", mode: "with_deps", hostOS: "darwin", arch: "arm64", allowHost: true,
			want: runCfg{mode: "with_deps"},
		},
		{
			name: "empty explicit no-deps mode", mode: "with_no_deps", hostOS: "linux", arch: "arm64",
			want: runCfg{mode: "with_no_deps"},
		},
		{
			name: "empty bad mode", mode: "bogus", hostOS: "darwin", arch: "arm64",
			wantErr: "with_deps or with_no_deps",
		},
		{
			name: "darwin defaults to VM per install", platform: "darwin-arm64", hostOS: "darwin", arch: "arm64",
			want: runCfg{mode: "with_no_deps", darwinVM: true},
		},
		{
			name: "darwin explicit no-deps", platform: "darwin-arm64", mode: "with_no_deps", hostOS: "darwin", arch: "arm64", virt: true,
			want: runCfg{mode: "with_no_deps", darwinVM: true},
		},
		{
			name: "darwin with_deps on darwin virt", platform: "darwin-arm64", mode: "with_deps", hostOS: "darwin", arch: "arm64", virt: true,
			want: runCfg{mode: "with_deps"},
		},
		{
			name: "darwin with_deps on bare metal", platform: "darwin-arm64", mode: "with_deps", hostOS: "darwin", arch: "arm64",
			wantErr: "installs never run on the bare-metal host",
		},
		{
			name: "darwin with_deps on bare metal allowed", platform: "darwin-arm64", mode: "with_deps", hostOS: "darwin", arch: "arm64", allowHost: true,
			want: runCfg{mode: "with_deps"},
		},
		{
			name: "darwin on linux", platform: "darwin-arm64", hostOS: "linux", arch: "arm64",
			wantErr: "needs a darwin-arm64 host",
		},
		{
			name: "darwin bad mode", platform: "darwin-arm64", mode: "bogus", hostOS: "darwin", arch: "arm64",
			wantErr: "with_deps or with_no_deps",
		},
		{
			name: "linux-amd64 on darwin-arm64", platform: "linux-amd64", hostOS: "darwin", arch: "arm64",
			want: runCfg{mode: "with_no_deps", linuxArch: "amd64"},
		},
		{
			name: "linux-arm64 on darwin-arm64", platform: "linux-arm64", hostOS: "darwin", arch: "arm64",
			want: runCfg{mode: "with_no_deps", linuxArch: "arm64"},
		},
		{
			name: "linux host same arch", platform: "linux-arm64", hostOS: "linux", arch: "arm64",
			want: runCfg{mode: "with_no_deps", linuxArch: "arm64"},
		},
		{
			name: "linux host cross arch", platform: "linux-amd64", hostOS: "linux", arch: "arm64",
			wantErr: "only the host platform (linux-arm64) is testable on linux hosts",
		},
		{
			name: "linux mode conflict", platform: "linux-amd64", mode: "with_deps", hostOS: "darwin", arch: "arm64",
			wantErr: "conflicts with platform linux-amd64",
		},
		{
			name: "unsupported platform", platform: "darwin-amd64", hostOS: "darwin", arch: "arm64",
			wantErr: "want darwin-arm64, linux-arm64 or linux-amd64",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := resolveRunCfg(c.platform, c.mode, c.hostOS, c.arch, c.virt, c.allowHost)
			if c.wantErr != "" {
				require.ErrorContains(t, err, c.wantErr)
				return
			}
			require.NoError(t, err)
			require.Equal(t, c.want, got)
		})
	}
}

// [<] 🤖🤖
