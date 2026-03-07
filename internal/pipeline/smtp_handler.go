package pipeline

import (
	"context"
	"fmt"
	"log"
	"net"
	"net/smtp"
	"strings"
)

// SmtpSendHandler implements NodeHandler for n8n-nodes-base.emailSend.
// It sends emails via SMTP for each input item.
type SmtpSendHandler struct{}

func (h *SmtpSendHandler) ExecuteNode(ctx context.Context, node *Node, input []NodeItem, creds map[string]map[string]string) (*NodeOutput, error) {
	// Resolve SMTP credentials
	smtpCreds := creds["smtp"]
	if smtpCreds == nil {
		for _, c := range creds {
			if c["smtp_host"] != "" {
				smtpCreds = c
				break
			}
		}
	}
	if smtpCreds == nil {
		return nil, fmt.Errorf("email send: no SMTP credentials found")
	}

	smtpHost := smtpCreds["smtp_host"]
	smtpUser := smtpCreds["smtp_user"]
	smtpPass := smtpCreds["smtp_pass"]
	if smtpHost == "" || smtpUser == "" || smtpPass == "" {
		return nil, fmt.Errorf("email send: incomplete SMTP credentials")
	}

	smtpAddr := smtpHost
	hostOnly := smtpHost
	if _, _, err := net.SplitHostPort(smtpHost); err != nil {
		smtpAddr = smtpHost + ":587"
	} else {
		hostOnly, _, _ = net.SplitHostPort(smtpHost)
	}

	out := &NodeOutput{Items: make([][]NodeItem, 1)}

	for _, item := range input {
		if ctx.Err() != nil {
			break
		}

		// Resolve parameters against current item
		params := ResolveNodeParams(node.Parameters, item)

		toEmail, _ := params["toEmail"].(string)
		fromEmail, _ := params["fromEmail"].(string)
		subject, _ := params["subject"].(string)
		emailType, _ := params["emailType"].(string)

		if toEmail == "" {
			log.Printf("[email send] skipping item: no toEmail")
			continue
		}
		if fromEmail == "" {
			fromEmail = smtpUser
		}

		// Build email body
		var body string
		var contentType string
		if emailType == "html" {
			body, _ = params["html"].(string)
			contentType = "text/html; charset=UTF-8"
		} else {
			body, _ = params["text"].(string)
			if body == "" {
				body, _ = params["body"].(string)
			}
			contentType = "text/plain; charset=UTF-8"
		}

		recipients := strings.Split(toEmail, ",")
		for i := range recipients {
			recipients[i] = strings.TrimSpace(recipients[i])
		}

		msg := fmt.Sprintf("From: %s\r\nTo: %s\r\nSubject: %s\r\nMIME-Version: 1.0\r\nContent-Type: %s\r\n\r\n%s",
			fromEmail, toEmail, subject, contentType, body)

		auth := smtp.PlainAuth("", smtpUser, smtpPass, hostOnly)
		if err := smtp.SendMail(smtpAddr, auth, fromEmail, recipients, []byte(msg)); err != nil {
			log.Printf("[email send] failed to send to %s: %v", toEmail, err)
			continue
		}

		log.Printf("[email send] sent to %s: %s", toEmail, subject)
		result := NodeItem(make(map[string]any))
		for k, v := range item {
			result[k] = v
		}
		result["_sent"] = true
		result["_sentTo"] = toEmail
		out.Items[0] = append(out.Items[0], result)
	}

	return out, nil
}
