package socket

import (
	"errors"
	"testing"
)

func TestIsENOBUFS(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{name: "nil", err: nil, want: false},
		{name: "libpcap text", err: errors.New("send: No buffer space available"), want: true},
		{name: "errno token", err: errors.New("write failed: ENOBUFS"), want: true},
		{name: "other error", err: errors.New("send: network is unreachable"), want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isENOBUFS(tt.err); got != tt.want {
				t.Fatalf("isENOBUFS() = %v, want %v", got, tt.want)
			}
		})
	}
}
