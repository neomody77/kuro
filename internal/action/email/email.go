// Package email implements email fetch and send actions.
package email

import (
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"net"
	"net/smtp"
	"strconv"
	"strings"
	"time"
)

// FetchAction connects to an IMAP server and fetches emails matching criteria.
type FetchAction struct{}

func (a *FetchAction) Execute(ctx context.Context, params map[string]any, creds map[string]string) (any, error) {
	host := creds["imap_host"]
	user := creds["imap_user"]
	pass := creds["imap_pass"]
	if host == "" || user == "" || pass == "" {
		return nil, fmt.Errorf("email.fetch: missing IMAP credentials (imap_host, imap_user, imap_pass)")
	}

	// Default port if not specified.
	if _, _, err := net.SplitHostPort(host); err != nil {
		host = host + ":993"
	}

	subjectMatch, _ := params["subject_match"].(string)
	fromMatch, _ := params["from"].(string)
	sinceStr, _ := params["since"].(string)
	limitF, _ := params["limit"].(float64)
	limit := int(limitF)
	if limit <= 0 {
		limit = 50
	}

	since := parseSince(sinceStr)

	conn, err := tls.DialWithDialer(&net.Dialer{Timeout: 15 * time.Second}, "tcp", host, &tls.Config{})
	if err != nil {
		return nil, fmt.Errorf("email.fetch: connect: %w", err)
	}
	defer conn.Close()

	c := &imapClient{conn: conn}

	// Read greeting.
	if _, err := c.readLine(); err != nil {
		return nil, fmt.Errorf("email.fetch: greeting: %w", err)
	}

	// Login.
	if err := c.command("LOGIN", fmt.Sprintf("%q %q", user, pass)); err != nil {
		return nil, fmt.Errorf("email.fetch: login: %w", err)
	}

	// Select INBOX.
	if err := c.command("SELECT", "INBOX"); err != nil {
		return nil, fmt.Errorf("email.fetch: select inbox: %w", err)
	}

	// Build SEARCH criteria.
	criteria := "ALL"
	var parts []string
	if !since.IsZero() {
		parts = append(parts, fmt.Sprintf("SINCE %s", since.Format("02-Jan-2006")))
	}
	if fromMatch != "" {
		parts = append(parts, fmt.Sprintf("FROM %q", fromMatch))
	}
	if subjectMatch != "" {
		parts = append(parts, fmt.Sprintf("SUBJECT %q", subjectMatch))
	}
	if len(parts) > 0 {
		criteria = strings.Join(parts, " ")
	}

	resp, err := c.commandResp("SEARCH", criteria)
	if err != nil {
		return nil, fmt.Errorf("email.fetch: search: %w", err)
	}

	uids := parseSearchResponse(resp)
	if len(uids) > limit {
		uids = uids[len(uids)-limit:]
	}

	var messages []map[string]any
	for _, uid := range uids {
		if ctx.Err() != nil {
			break
		}
		body, err := c.commandResp("FETCH", fmt.Sprintf("%d BODY[HEADER.FIELDS (FROM SUBJECT DATE)]", uid))
		if err != nil {
			continue
		}
		msg := parseHeaders(body)
		msg["uid"] = uid

		// Fetch body text.
		textResp, err := c.commandResp("FETCH", fmt.Sprintf("%d BODY[TEXT]", uid))
		if err == nil {
			msg["body"] = extractBody(textResp)
		}

		messages = append(messages, msg)
	}

	// Logout.
	_ = c.command("LOGOUT", "")

	return map[string]any{
		"messages": messages,
		"count":    len(messages),
	}, nil
}

// SendAction sends an email via SMTP.
type SendAction struct{}

func (a *SendAction) Execute(ctx context.Context, params map[string]any, creds map[string]string) (any, error) {
	host := creds["smtp_host"]
	user := creds["smtp_user"]
	pass := creds["smtp_pass"]
	if host == "" || user == "" || pass == "" {
		return nil, fmt.Errorf("email.send: missing SMTP credentials (smtp_host, smtp_user, smtp_pass)")
	}

	to, _ := params["to"].(string)
	subject, _ := params["subject"].(string)
	body, _ := params["body"].(string)
	from := user

	if fromParam, ok := params["from"].(string); ok && fromParam != "" {
		from = fromParam
	}

	if to == "" {
		return nil, fmt.Errorf("email.send: 'to' parameter is required")
	}

	// Default port if not specified.
	smtpHost := host
	smtpAddr := host
	if _, _, err := net.SplitHostPort(host); err != nil {
		smtpAddr = host + ":587"
		smtpHost = host
	} else {
		smtpHost, _, _ = net.SplitHostPort(host)
	}

	msg := fmt.Sprintf("From: %s\r\nTo: %s\r\nSubject: %s\r\nMIME-Version: 1.0\r\nContent-Type: text/plain; charset=UTF-8\r\n\r\n%s",
		from, to, subject, body)

	auth := smtp.PlainAuth("", user, pass, smtpHost)

	recipients := strings.Split(to, ",")
	for i := range recipients {
		recipients[i] = strings.TrimSpace(recipients[i])
	}

	if err := smtp.SendMail(smtpAddr, auth, from, recipients, []byte(msg)); err != nil {
		return nil, fmt.Errorf("email.send: %w", err)
	}

	return map[string]any{
		"sent": true,
		"to":   to,
	}, nil
}

// Minimal IMAP client.
type imapClient struct {
	conn io.ReadWriter
	tag  int
}

func (c *imapClient) readLine() (string, error) {
	var buf [4096]byte
	n, err := c.conn.Read(buf[:])
	if err != nil {
		return "", err
	}
	return string(buf[:n]), nil
}

func (c *imapClient) command(cmd, args string) error {
	_, err := c.commandResp(cmd, args)
	return err
}

func (c *imapClient) commandResp(cmd, args string) (string, error) {
	c.tag++
	tag := fmt.Sprintf("A%03d", c.tag)

	line := fmt.Sprintf("%s %s", tag, cmd)
	if args != "" {
		line += " " + args
	}
	line += "\r\n"

	if _, err := io.WriteString(c.conn, line); err != nil {
		return "", err
	}

	// Read until we get our tagged response.
	var full strings.Builder
	for {
		resp, err := c.readLine()
		if err != nil {
			return full.String(), err
		}
		full.WriteString(resp)
		if strings.Contains(resp, tag+" OK") || strings.Contains(resp, tag+" NO") || strings.Contains(resp, tag+" BAD") {
			break
		}
	}

	result := full.String()
	if strings.Contains(result, tag+" NO") || strings.Contains(result, tag+" BAD") {
		return result, fmt.Errorf("IMAP %s failed: %s", cmd, result)
	}
	return result, nil
}

func parseSince(s string) time.Time {
	switch strings.ToLower(s) {
	case "today":
		now := time.Now()
		return time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	case "week":
		return time.Now().AddDate(0, 0, -7)
	case "month":
		return time.Now().AddDate(0, -1, 0)
	default:
		t, err := time.Parse("2006-01-02", s)
		if err != nil {
			return time.Time{}
		}
		return t
	}
}

func parseSearchResponse(resp string) []int {
	var uids []int
	for _, line := range strings.Split(resp, "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "* SEARCH") {
			continue
		}
		fields := strings.Fields(line)
		for _, f := range fields[2:] {
			if uid, err := strconv.Atoi(f); err == nil {
				uids = append(uids, uid)
			}
		}
	}
	return uids
}

func parseHeaders(resp string) map[string]any {
	msg := map[string]any{}
	for _, line := range strings.Split(resp, "\r\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "From:") {
			msg["from"] = strings.TrimSpace(strings.TrimPrefix(line, "From:"))
		} else if strings.HasPrefix(line, "Subject:") {
			msg["subject"] = strings.TrimSpace(strings.TrimPrefix(line, "Subject:"))
		} else if strings.HasPrefix(line, "Date:") {
			msg["date"] = strings.TrimSpace(strings.TrimPrefix(line, "Date:"))
		}
	}
	return msg
}

func extractBody(resp string) string {
	// The body text is between the FETCH response header and the closing paren.
	lines := strings.Split(resp, "\r\n")
	var body strings.Builder
	inBody := false
	for _, line := range lines {
		if strings.Contains(line, "BODY[TEXT]") {
			inBody = true
			continue
		}
		if inBody {
			if strings.HasPrefix(line, ")") || strings.Contains(line, " OK FETCH") {
				break
			}
			body.WriteString(line)
			body.WriteString("\n")
		}
	}
	return strings.TrimSpace(body.String())
}
