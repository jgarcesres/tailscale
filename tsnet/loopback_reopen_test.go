// Copyright (c) Tailscale Inc & AUTHORS
// SPDX-License-Identifier: BSD-3-Clause

package tsnet

import (
	"context"
	"net"
	"net/http"
	"testing"
	"time"
)

// TestLoopbackReopensDeadListener covers the OS reclaiming the loopback
// listener (iOS does it to a suspended app): the next Loopback call must
// hand out a listener that answers, not the cached dead address.
func TestLoopbackReopensDeadListener(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	controlURL, _ := startControl(t)
	s1, _, _ := startServer(t, ctx, controlURL, "s1")

	addr, proxyCred, _, err := s1.Loopback()
	if err != nil {
		t.Fatal(err)
	}
	again, againCred, _, err := s1.Loopback()
	if err != nil {
		t.Fatal(err)
	}
	if again != addr || againCred != proxyCred {
		t.Fatalf("live listener: Loopback = %s, want the same %s", again, addr)
	}

	// What the OS does to it, from the outside.
	s1.loopbackMu.Lock()
	s1.loopbackListener.Close()
	s1.loopbackMu.Unlock()
	if c, err := net.Dial("tcp", addr); err == nil {
		c.Close()
		t.Fatal("closed listener still accepts connections")
	}

	fresh, _, localAPICred, err := s1.Loopback()
	if err != nil {
		t.Fatal(err)
	}
	if fresh == addr {
		t.Fatalf("Loopback returned the dead address %s again", addr)
	}
	req, err := http.NewRequestWithContext(ctx, "GET", "http://"+fresh+"/localapi/v0/status", nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Sec-Tailscale", "localapi")
	req.SetBasicAuth("", localAPICred)
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("new listener: %v", err)
	}
	res.Body.Close()
	if res.StatusCode != 200 {
		t.Fatalf("new listener: GET status = %d, want 200", res.StatusCode)
	}
}
