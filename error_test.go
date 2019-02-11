package proxy

import "testing"

func TestParseError(t *testing.T) {
	e := parseError([]byte(`{"errorMessage":"fork/exec /var/task/ssm-sign-proxy: no such file or directory","errorType":"PathError"}`))
	if e.Error() != "fork/exec /var/task/ssm-sign-proxy: no such file or directory" {
		t.Errorf("want fork/exec /var/task/ssm-sign-proxy: no such file or directory, got %s", e.Error())
	}
}
