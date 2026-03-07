package pipeline

import (
	"context"
	"crypto/tls"
	"fmt"
	"log"
	"net"
	"strings"
	"time"
)

// ImapIdleWatcher maintains a persistent IMAP connection using IDLE
// to get near-instant notification of new emails instead of polling.
type ImapIdleWatcher struct {
	host     string
	user     string
	pass     string
	onNotify func() // called when new mail arrives
}

// NewImapIdleWatcher creates a watcher that calls onNotify when new mail arrives.
func NewImapIdleWatcher(host, user, pass string, onNotify func()) *ImapIdleWatcher {
	if _, _, err := net.SplitHostPort(host); err != nil {
		host = host + ":993"
	}
	return &ImapIdleWatcher{host: host, user: user, pass: pass, onNotify: onNotify}
}

// Run maintains a persistent IMAP IDLE connection. It reconnects on errors.
// Blocks until ctx is canceled.
func (w *ImapIdleWatcher) Run(ctx context.Context) {
	for {
		if ctx.Err() != nil {
			return
		}
		err := w.idleLoop(ctx)
		if ctx.Err() != nil {
			return
		}
		log.Printf("[imap idle] connection lost: %v, reconnecting in 10s", err)
		select {
		case <-ctx.Done():
			return
		case <-time.After(10 * time.Second):
		}
	}
}

// idleLoop connects, authenticates, and enters IDLE mode.
// Returns on error (caller will reconnect).
func (w *ImapIdleWatcher) idleLoop(ctx context.Context) error {
	conn, err := tls.DialWithDialer(&net.Dialer{Timeout: 15 * time.Second}, "tcp", w.host, &tls.Config{})
	if err != nil {
		return fmt.Errorf("connect: %w", err)
	}
	defer conn.Close()

	c := &imapConn{conn: conn}

	// Read greeting
	if _, err := c.readLine(); err != nil {
		return fmt.Errorf("greeting: %w", err)
	}

	// Login
	if err := c.cmd("LOGIN", fmt.Sprintf("%q %q", w.user, w.pass)); err != nil {
		return fmt.Errorf("login: %w", err)
	}

	// Select INBOX
	if err := c.cmd("SELECT", "INBOX"); err != nil {
		return fmt.Errorf("select: %w", err)
	}

	log.Printf("[imap idle] connected to %s, entering IDLE", w.host)

	// IDLE loop — RFC 2177 recommends re-issuing IDLE every 29 minutes
	for {
		if ctx.Err() != nil {
			_ = c.cmd("LOGOUT", "")
			return ctx.Err()
		}

		// Start IDLE
		c.tag++
		idleTag := fmt.Sprintf("A%03d", c.tag)
		idleLine := idleTag + " IDLE\r\n"
		if _, err := fmt.Fprint(c.conn, idleLine); err != nil {
			return fmt.Errorf("send IDLE: %w", err)
		}

		// Wait for + continuation response
		resp, err := c.readLine()
		if err != nil {
			return fmt.Errorf("idle continuation: %w", err)
		}
		if !strings.HasPrefix(resp, "+") {
			return fmt.Errorf("unexpected idle response: %s", resp)
		}

		// Now wait for untagged responses (new mail) or timeout (re-IDLE every 25 min)
		gotMail, err := w.waitForNotification(ctx, c, 25*time.Minute)

		// Send DONE to end IDLE regardless of what happened
		if _, writeErr := fmt.Fprint(c.conn, "DONE\r\n"); writeErr != nil {
			return fmt.Errorf("send DONE: %w", writeErr)
		}

		// Read the tagged OK response for IDLE
		doneResp, readErr := c.readLine()
		if readErr != nil {
			return fmt.Errorf("idle done response: %w", readErr)
		}
		// Consume any remaining untagged responses until we see the tag
		for !strings.Contains(doneResp, idleTag) {
			doneResp, readErr = c.readLine()
			if readErr != nil {
				return fmt.Errorf("idle done drain: %w", readErr)
			}
		}

		if err != nil {
			return err
		}

		if gotMail {
			log.Printf("[imap idle] new mail notification received")
			w.onNotify()
		}
		// If !gotMail, it was a timeout — just re-IDLE
	}
}

// waitForNotification waits for an IMAP untagged EXISTS/RECENT response
// or a timeout. Returns (true, nil) if new mail, (false, nil) on timeout.
func (w *ImapIdleWatcher) waitForNotification(ctx context.Context, c *imapConn, timeout time.Duration) (bool, error) {
	type readResult struct {
		data string
		err  error
	}
	ch := make(chan readResult, 1)

	// Read in a goroutine since Read() blocks
	go func() {
		data, err := c.readLine()
		ch <- readResult{data, err}
	}()

	timer := time.NewTimer(timeout)
	defer timer.Stop()

	for {
		select {
		case <-ctx.Done():
			return false, ctx.Err()
		case <-timer.C:
			// Timeout — re-IDLE
			return false, nil
		case result := <-ch:
			if result.err != nil {
				return false, fmt.Errorf("idle read: %w", result.err)
			}
			// Check for EXISTS or RECENT notifications
			line := result.data
			if strings.Contains(line, "EXISTS") || strings.Contains(line, "RECENT") {
				return true, nil
			}
			// Other untagged response (FLAGS, EXPUNGE, etc.) — keep waiting
			go func() {
				data, err := c.readLine()
				ch <- readResult{data, err}
			}()
		}
	}
}
