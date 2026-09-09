package git

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDetectGitRepositoryRoot(t *testing.T) {
	tests := []struct {
		name       string
		dotGitFile bool
	}{
		{
			name: "git directory",
		},
		{
			name:       "git worktree file",
			dotGitFile: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repoRoot := t.TempDir()
			dotGitPath := filepath.Join(repoRoot, ".git")
			if tt.dotGitFile {
				err := os.WriteFile(dotGitPath, []byte("gitdir: /path/to/common/git/dir\n"), 0o600)
				if err != nil {
					t.Fatal(err)
				}
			} else {
				err := os.Mkdir(dotGitPath, 0o700)
				if err != nil {
					t.Fatal(err)
				}
			}

			projectDir := filepath.Join(repoRoot, "path", "to", "project")
			if err := os.MkdirAll(projectDir, 0o700); err != nil {
				t.Fatal(err)
			}

			actual, err := DetectGitRepositoryRoot(projectDir)
			if err != nil {
				t.Fatal(err)
			}
			if actual != repoRoot {
				t.Fatalf("expected repository root %q, got %q", repoRoot, actual)
			}
		})
	}
}
