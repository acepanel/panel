package rule

import (
	"strings"
	"testing"

	"github.com/libtnb/assert/check"
)

func TestCronPattern(t *testing.T) {
	re := newCron(nil).re

	for _, spec := range []string{
		"* * * * *",
		"*/5 * * * *",
		"30 4 * * *",
		"0 0 1,15 * *",
		"0 9-18 * * 1-5",
		"0 9-18/2 * * 1-5",
		"*/15,45 * * * *",
		"0 0 * * MON-FRI",
		"0 0 1 JAN,JUL *",
		// 6 字段的秒级任务
		"* * * * * *",
		"*/10 * * * * *",
		"@daily",
		"@reboot",
		"@every 1h30m",
		" */5 * * * * ",
	} {
		check.True(t, re.MatchString(strings.TrimSpace(spec)), check.Msgf("应通过: %q", spec))
	}

	for _, spec := range []string{
		"",
		"12345",
		"* * * *",
		"abc 1 2 3 4 5 xyz",
		"* * * * * * * *",
		"*/5 * * * *; rm -rf /",
		"60s",
		"@every",
		"@yearly extra",
		"a b c d e",
		"1--2 * * * *",
		"*/ * * * *",
	} {
		check.False(t, re.MatchString(strings.TrimSpace(spec)), check.Msgf("应拒绝: %q", spec))
	}
}
