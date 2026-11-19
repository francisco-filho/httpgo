package config

import (
	"fmt"
	"strings"
	"testing"
	"time"
)

func TestTimeoutExactMilliseconds(t *testing.T) {
	for _, tc := range []struct {
		source string
		want   time.Duration
	}{
		{"0.001", time.Millisecond}, {"86400", 24 * time.Hour}, {"2.5", 2500 * time.Millisecond}, {"1e-3", time.Millisecond}, {"2.5000", 2500 * time.Millisecond}, {"1.001", 1001 * time.Millisecond}, {"+1", time.Second}, {"0x1", time.Second}, {".001", time.Millisecond}, {"1.", time.Second}, {"86400000e-3", 24 * time.Hour},
	} {
		t.Run(tc.source, func(t *testing.T) {
			got, err := decode("timeout.yaml", []byte("version: 1\ndefaults: {timeout: "+tc.source+"}"))
			if err != nil || got.Timeout != tc.want {
				t.Fatalf("got=%s want=%s err=%v", got.Timeout, tc.want, err)
			}
		})
	}
	for _, source := range []string{"0", "-0.001", "86400.001", "0.0001", "1.0001", "2.5000000000000000001", ".inf", "-.inf", ".nan", "true", "null", "[]", "{}", "'2.5'", "2s", "1e999", "1e-999", "1e999999999999999999999999999999"} {
		t.Run("reject "+source, func(t *testing.T) {
			_, err := decode("timeout.yaml", []byte("version: 1\ndefaults: {timeout: "+source+"}"))
			if err == nil || !strings.Contains(err.Error(), "timeout") || !strings.Contains(err.Error(), ":2:") {
				t.Fatalf("accepted %s err=%v", source, err)
			}
		})
	}
	for _, defaults := range []string{"", "defaults: {}"} {
		got, err := decode("timeout.yaml", fmt.Appendf(nil, "version: 1\n%s", defaults))
		if err != nil || got.Timeout != DefaultTimeout {
			t.Fatalf("got=%s err=%v", got.Timeout, err)
		}
	}
}
