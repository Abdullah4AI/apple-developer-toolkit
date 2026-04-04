package shared

import (
	"fmt"
	"strings"
)

// MultiStringFlag collects repeatable string flag values while rejecting empty
// entries so callers can bind it directly with flag.FlagSet.Var.
type MultiStringFlag []string

func (m *MultiStringFlag) String() string {
	if m == nil {
		return ""
	}
	return strings.Join(*m, ",")
}

func (m *MultiStringFlag) Set(value string) error {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return fmt.Errorf("value cannot be empty")
	}
	*m = append(*m, trimmed)
	return nil
}
