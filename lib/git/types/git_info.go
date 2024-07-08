package types

type GitInfo struct {
	Url    *GitUrl `json:"url"`
	Ref    *GitRef `json:"ref"`
	SubDir string  `json:"subDir"`
	Commit string  `json:"commit"`
	Dirty  bool    `json:"dirty"`
}
