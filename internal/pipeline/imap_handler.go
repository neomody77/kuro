package pipeline

import (
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"log"
	"net"
	"strconv"
	"strings"
	"time"
)

// ImapTriggerHandler implements NodeHandler for n8n-nodes-base.emailReadImap.
// It fetches UNSEEN emails via IMAP and emits each as a NodeItem.
type ImapTriggerHandler struct{}

func (h *ImapTriggerHandler) ExecuteNode(ctx context.Context, node *Node, _ []NodeItem, creds map[string]map[string]string) (*NodeOutput, error) {
	// Resolve IMAP credentials
	imapCreds := creds["imap"]
	if imapCreds == nil {
		// Try looking for any credential that has imap_host
		for _, c := range creds {
			if c["imap_host"] != "" {
				imapCreds = c
				break
			}
		}
	}
	if imapCreds == nil {
		return nil, fmt.Errorf("imap trigger: no IMAP credentials found")
	}

	host := imapCreds["imap_host"]
	user := imapCreds["imap_user"]
	pass := imapCreds["imap_pass"]
	if host == "" || user == "" || pass == "" {
		return nil, fmt.Errorf("imap trigger: incomplete IMAP credentials")
	}

	if _, _, err := net.SplitHostPort(host); err != nil {
		host = host + ":993"
	}

	// Get lastMessageUid from node's staticData
	var lastUID int
	if sd, ok := node.Parameters["_staticData"].(map[string]any); ok {
		if uid, ok := sd["lastMessageUid"].(float64); ok {
			lastUID = int(uid)
		}
	}

	conn, err := tls.DialWithDialer(&net.Dialer{Timeout: 15 * time.Second}, "tcp", host, &tls.Config{})
	if err != nil {
		return nil, fmt.Errorf("imap trigger: connect: %w", err)
	}
	defer conn.Close()

	c := &imapConn{conn: conn}

	if _, err := c.readLine(); err != nil {
		return nil, fmt.Errorf("imap trigger: greeting: %w", err)
	}
	if err := c.cmd("LOGIN", fmt.Sprintf("%q %q", user, pass)); err != nil {
		return nil, fmt.Errorf("imap trigger: login: %w", err)
	}
	if err := c.cmd("SELECT", "INBOX"); err != nil {
		return nil, fmt.Errorf("imap trigger: select: %w", err)
	}

	// Search for UNSEEN messages using UID SEARCH (returns real UIDs, not sequence numbers)
	var resp string
	if lastUID > 0 {
		// Only search UIDs greater than lastUID
		resp, err = c.cmdResp("UID SEARCH", fmt.Sprintf("UID %d:* UNSEEN", lastUID+1))
	} else {
		resp, err = c.cmdResp("UID SEARCH", "UNSEEN")
	}
	if err != nil {
		return nil, fmt.Errorf("imap trigger: search: %w", err)
	}

	newUIDs := parseSearchUIDs(resp)
	// Filter out the lastUID itself (UID range is inclusive)
	var filtered []int
	for _, uid := range newUIDs {
		if uid > lastUID {
			filtered = append(filtered, uid)
		}
	}
	newUIDs = filtered

	out := &NodeOutput{Items: make([][]NodeItem, 1)}
	maxUID := lastUID

	for _, uid := range newUIDs {
		if ctx.Err() != nil {
			break
		}
		if uid > maxUID {
			maxUID = uid
		}

		// Fetch headers including To (using UID FETCH)
		headerResp, err := c.cmdResp("UID FETCH", fmt.Sprintf("%d BODY[HEADER.FIELDS (FROM TO SUBJECT DATE)]", uid))
		if err != nil {
			log.Printf("[imap trigger] fetch headers uid=%d: %v", uid, err)
			continue
		}
		item := parseEmailHeaders(headerResp)
		item["uid"] = uid

		// Fetch text body
		textResp, err := c.cmdResp("UID FETCH", fmt.Sprintf("%d BODY[TEXT]", uid))
		if err == nil {
			item["textPlain"] = extractTextBody(textResp)
		}

		// Fetch HTML body
		htmlResp, err := c.cmdResp("UID FETCH", fmt.Sprintf("%d BODY[1]", uid))
		if err == nil {
			body := extractTextBody(htmlResp)
			if strings.Contains(body, "<") {
				item["textHtml"] = body
			}
		}

		out.Items[0] = append(out.Items[0], NodeItem(item))
	}

	// Store the new lastUID for static data update
	if maxUID > lastUID {
		if node.Parameters == nil {
			node.Parameters = make(map[string]any)
		}
		node.Parameters["_newLastUID"] = maxUID
	}

	_ = c.cmd("LOGOUT", "")

	log.Printf("[imap trigger] fetched %d new messages (lastUID: %d -> %d)", len(out.Items[0]), lastUID, maxUID)
	return out, nil
}

// --- minimal IMAP client ---

type imapConn struct {
	conn io.ReadWriter
	tag  int
}

func (c *imapConn) readLine() (string, error) {
	var buf [8192]byte
	n, err := c.conn.Read(buf[:])
	if err != nil {
		return "", err
	}
	return string(buf[:n]), nil
}

func (c *imapConn) cmd(cmd, args string) error {
	_, err := c.cmdResp(cmd, args)
	return err
}

func (c *imapConn) cmdResp(cmd, args string) (string, error) {
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

func parseSearchUIDs(resp string) []int {
	var uids []int
	for _, line := range strings.Split(resp, "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "* SEARCH") {
			continue
		}
		for _, f := range strings.Fields(line)[2:] {
			if uid, err := strconv.Atoi(f); err == nil {
				uids = append(uids, uid)
			}
		}
	}
	return uids
}

func parseEmailHeaders(resp string) map[string]any {
	msg := map[string]any{}
	for _, line := range strings.Split(resp, "\r\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "From:") {
			msg["from"] = strings.TrimSpace(strings.TrimPrefix(line, "From:"))
		} else if strings.HasPrefix(line, "To:") {
			msg["to"] = strings.TrimSpace(strings.TrimPrefix(line, "To:"))
		} else if strings.HasPrefix(line, "Subject:") {
			msg["subject"] = strings.TrimSpace(strings.TrimPrefix(line, "Subject:"))
		} else if strings.HasPrefix(line, "Date:") {
			msg["date"] = strings.TrimSpace(strings.TrimPrefix(line, "Date:"))
		}
	}
	return msg
}

func extractTextBody(resp string) string {
	lines := strings.Split(resp, "\r\n")
	var body strings.Builder
	inBody := false
	for _, line := range lines {
		if strings.Contains(line, "BODY[") {
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
