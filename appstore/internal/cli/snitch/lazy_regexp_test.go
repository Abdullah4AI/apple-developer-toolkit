package snitch

import (
	"regexp"
	"testing"
)

func TestLazyPatternsCompile(t *testing.T) {
	if len(lazyPatterns) == 0 {
		t.Fatal("no lazy patterns registered")
	}
	for _, l := range lazyPatterns {
		if _, err := regexp.Compile(l.expr); err != nil {
			t.Errorf("pattern %q does not compile: %v", l.expr, err)
		}
	}
}
