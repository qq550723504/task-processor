package mail

import (
	"bufio"
	"context"
	"fmt"
	"github.com/google/uuid"
	"net"
	"strconv"
	"strings"
	flow "task-processor/internal/organization/membership/inviteflow"
	"testing"
	"time"
)

func TestRejectInsecureOrUntrustedNotificationConfig(t *testing.T) {
	base := Config{Host: "smtp.example.test", Port: 587, From: "invitations@example.test", PublicOrigin: "https://app.example.test"}
	for _, change := range []func(*Config){func(c *Config) { c.LocalPlaintext = true }, func(c *Config) { c.PublicOrigin = "https://app.example.test/path" }, func(c *Config) { c.From = "bad\r\nBcc: a@example.test" }, func(c *Config) { c.Username = "user" }} {
		cfg := base
		change(&cfg)
		if _, err := New(cfg); err == nil {
			t.Fatal("invalid configuration accepted")
		}
	}
}
func TestSMTPActualMessageAndStalledDataCancellation(t *testing.T) {
	for _, stall := range []bool{false, true} {
		t.Run(fmt.Sprint(stall), func(t *testing.T) {
			listener, err := net.Listen("tcp", "127.0.0.1:0")
			if err != nil {
				t.Fatal(err)
			}
			defer listener.Close()
			message := make(chan string, 1)
			done := make(chan struct{})
			go func() {
				defer close(done)
				conn, err := listener.Accept()
				if err != nil {
					return
				}
				defer conn.Close()
				fmt.Fprint(conn, "220 localhost ESMTP\r\n")
				reader := bufio.NewReader(conn)
				var body strings.Builder
				data := false
				for {
					line, err := reader.ReadString('\n')
					if err != nil {
						return
					}
					if data {
						if line == ".\r\n" {
							message <- body.String()
							data = false
							if stall {
								_, _ = reader.ReadString('\n')
								return
							}
							fmt.Fprint(conn, "250 queued\r\n")
						} else {
							body.WriteString(line)
						}
						continue
					}
					switch {
					case strings.HasPrefix(line, "EHLO"):
						fmt.Fprint(conn, "250 localhost\r\n")
					case strings.HasPrefix(line, "DATA"):
						data = true
						fmt.Fprint(conn, "354 go\r\n")
					case strings.HasPrefix(line, "QUIT"):
						fmt.Fprint(conn, "221 bye\r\n")
						return
					default:
						fmt.Fprint(conn, "250 OK\r\n")
					}
				}
			}()
			_, port, _ := net.SplitHostPort(listener.Addr().String())
			n, _ := strconv.Atoi(port)
			sender, err := New(Config{Host: "127.0.0.1", Port: n, LocalPlaintext: true, From: "invitations@example.test", PublicOrigin: "https://localhost:22544"})
			if err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
			defer cancel()
			inv := flow.Invitation{ID: uuid.NewString(), Contact: "recipient@example.test", OrganizationName: "测试企业", Role: "listingkit_viewer", ExpiresAt: time.Now().Add(time.Hour)}
			start := time.Now()
			err = sender.Send(ctx, inv)
			if stall && err == nil || !stall && err != nil {
				t.Fatal("result", err)
			}
			if time.Since(start) > 2*time.Second {
				t.Fatal("SMTP DATA did not cancel")
			}
			select {
			case body := <-message:
				if !strings.Contains(body, inv.ID) {
					t.Fatal("missing configured invitation link")
				}
			case <-time.After(time.Second):
				t.Fatal("no actual message")
			}
			select {
			case <-done:
			case <-time.After(time.Second):
				t.Fatal("connection did not close")
			}
		})
	}
}
