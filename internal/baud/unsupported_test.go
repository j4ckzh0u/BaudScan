package baud

import (
	"errors"
	"syscall"
	"testing"
)

func TestUnsupported(t *testing.T) {
	if !Unsupported(syscall.EINVAL) {
		t.Error("EINVAL is not reported as unsupported")
	}
	if Unsupported(errors.New("other")) {
		t.Error("a plain error is reported as unsupported")
	}
	if Unsupported(nil) {
		t.Error("nil is reported as unsupported")
	}
	if got := FriendlyError(syscall.EINVAL); got != "此设备不支持该波特率" {
		t.Errorf("FriendlyError(EINVAL) = %q", got)
	}
}
