package app

import "testing"

// A token in the config wins; otherwise the environment is consulted in order,
// so the credential can stay out of both the config file and shell history.
func TestGithubToken(t *testing.T) {
	tests := []struct {
		name       string
		configured string
		env        map[string]string
		want       string
	}{
		{
			name: "nothing configured anywhere",
			want: "",
		},
		{
			name:       "config only",
			configured: "from-config",
			want:       "from-config",
		},
		{
			name: "GH_TOKEN",
			env:  map[string]string{"GH_TOKEN": "from-gh"},
			want: "from-gh",
		},
		{
			name: "GITHUB_TOKEN",
			env:  map[string]string{"GITHUB_TOKEN": "from-github"},
			want: "from-github",
		},
		{
			name: "guppy's own variable wins over the generic ones",
			env: map[string]string{
				"GUPPY_GITHUB_TOKEN": "from-guppy",
				"GH_TOKEN":           "from-gh",
				"GITHUB_TOKEN":       "from-github",
			},
			want: "from-guppy",
		},
		{
			name: "GH_TOKEN wins over GITHUB_TOKEN",
			env: map[string]string{
				"GH_TOKEN":     "from-gh",
				"GITHUB_TOKEN": "from-github",
			},
			want: "from-gh",
		},
		{
			name:       "config beats the environment",
			configured: "from-config",
			env:        map[string]string{"GH_TOKEN": "from-gh"},
			want:       "from-config",
		},
		{
			name: "an empty variable is skipped, not treated as set",
			env: map[string]string{
				"GH_TOKEN":     "",
				"GITHUB_TOKEN": "from-github",
			},
			want: "from-github",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Clear all of them first: the real environment may have one set.
			for _, key := range githubTokenEnv {
				t.Setenv(key, "")
			}
			for key, value := range tt.env {
				t.Setenv(key, value)
			}

			if got := githubToken(tt.configured); got != tt.want {
				t.Errorf("githubToken(%q) = %q, want %q", tt.configured, got, tt.want)
			}
		})
	}
}
