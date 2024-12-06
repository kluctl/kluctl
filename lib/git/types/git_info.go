package types

// GitInfo represents the result of BuildGitInfo, which gathers all info from a local cloned git repository
type GitInfo struct {
	Url    *GitUrl `json:"url"`
	Ref    *GitRef `json:"ref"`
	SubDir string  `json:"subDir"`
	Commit string  `json:"commit"`
	Dirty  bool    `json:"dirty"`
}
