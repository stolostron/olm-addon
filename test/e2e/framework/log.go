package framework

import (
	"testing"
	"time"
)

func Logf(t *testing.T, format string, args ...any) {
	t.Logf("%s: "+format, append([]any{time.Now().Format(time.RFC3339)}, args...)...)
}
