package run

import "strings"

const (
	RunModeCollab = "collab"
	RunModeSenate = "senate"
	RunModeRage   = "rage"
)

func normalizedRunMode(mode string) string {
	switch strings.ToLower(strings.TrimSpace(mode)) {
	case "", RunModeCollab:
		return RunModeCollab
	case RunModeSenate:
		return RunModeSenate
	case RunModeRage:
		return RunModeRage
	default:
		return strings.ToLower(strings.TrimSpace(mode))
	}
}

func NormalizeMode(mode string) string {
	return normalizedRunMode(mode)
}
