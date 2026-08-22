package http

import (
	"net/http"
	"testing"
)

func TestIsLoopback(t *testing.T) {
	t.Run("genuine loopback IPv4", func(t *testing.T) {
		r := &http.Request{RemoteAddr: "127.0.0.1:54321"}
		if !isLoopback(r) {
			t.Fatal("expected 127.0.0.1 to be loopback")
		}
	})

	t.Run("genuine loopback IPv6", func(t *testing.T) {
		r := &http.Request{RemoteAddr: "[::1]:54321"}
		if !isLoopback(r) {
			t.Fatal("expected ::1 to be loopback")
		}
	})

	t.Run("non-loopback peer", func(t *testing.T) {
		r := &http.Request{RemoteAddr: "192.168.1.5:54321"}
		if isLoopback(r) {
			t.Fatal("expected 192.168.1.5 to not be loopback")
		}
	})

	t.Run("forwarded header is not trusted", func(t *testing.T) {
		// A non-loopback peer spoofing X-Forwarded-For must NOT be treated as
		// loopback: isLoopback only inspects the raw TCP peer (RemoteAddr).
		r := &http.Request{
			RemoteAddr: "192.168.1.5:54321",
			Header:     http.Header{"X-Forwarded-For": []string{"127.0.0.1"}},
		}
		if isLoopback(r) {
			t.Fatal("X-Forwarded-For must not make a non-loopback peer loopback")
		}
	})

	t.Run("malformed remote addr", func(t *testing.T) {
		r := &http.Request{RemoteAddr: "not-an-addr"}
		if isLoopback(r) {
			t.Fatal("expected malformed RemoteAddr to not be loopback")
		}
	})
}
