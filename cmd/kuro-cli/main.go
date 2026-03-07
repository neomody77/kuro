package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/neomody77/kuro/internal/cli"
)

var client *cli.Client

func main() {
	client = cli.NewClient()

	if len(os.Args) < 2 {
		printUsage()
		os.Exit(0)
	}

	cmd := os.Args[1]
	args := os.Args[2:]

	var err error
	switch cmd {
	case "chat":
		err = cmdChat(args)
	case "session":
		err = cmdSession(args)
	case "pipeline", "p":
		err = cmdPipeline(args)
	case "doc", "d":
		err = cmdDoc(args)
	case "cred", "c":
		err = cmdCred(args)
	case "skill", "s":
		err = cmdSkill(args)
	case "log", "l":
		err = cmdLog(args)
	case "health":
		err = cmdHealth()
	case "help", "-h", "--help":
		printUsage()
	default:
		fmt.Fprintf(os.Stderr, "Unknown command: %s\n", cmd)
		printUsage()
		os.Exit(1)
	}

	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Print(`Kuro CLI - Personal automation assistant

Usage: kuro-cli <command> [args]

Commands:
  chat [-s session_id] [message]  Chat with Kuro (interactive REPL if no message)
  session <action>                Manage chat sessions (list, new, delete, history)
  pipeline|p <action>             Manage pipelines (list, get, run, delete)
  doc|d <action>                  Manage documents (list, get, put, delete, search)
  cred|c <action>                 Manage credentials (list, add, delete)
  skill|s <action>                Manage skills (list, exec)
  log|l <action>                  View execution logs (list, get)
  health                          Check server status

Environment:
  KURO_URL    Server URL (default: http://localhost:8080)
  KURO_TOKEN  Auth token (if USER_TOKENS is set on server)
`)
}

// --- Chat ---

var chatSessionID string

func cmdChat(args []string) error {
	// Parse -s flag
	rest := args
	for i := 0; i < len(rest); i++ {
		if rest[i] == "-s" && i+1 < len(rest) {
			chatSessionID = rest[i+1]
			rest = append(rest[:i], rest[i+2:]...)
			break
		}
	}

	if len(rest) > 0 {
		return chatSend(strings.Join(rest, " "))
	}
	// Interactive REPL
	if chatSessionID != "" {
		fmt.Printf("Kuro Chat [session: %s] (type 'exit' to quit)\n", chatSessionID)
	} else {
		fmt.Println("Kuro Chat (type 'exit' to quit)")
	}
	scanner := bufio.NewScanner(os.Stdin)
	for {
		fmt.Print("\n> ")
		if !scanner.Scan() {
			break
		}
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		if line == "exit" || line == "quit" {
			break
		}
		if err := chatSend(line); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		}
	}
	return nil
}

func chatSend(msg string) error {
	body := map[string]string{"message": msg}
	if chatSessionID != "" {
		body["session_id"] = chatSessionID
	}
	data, err := client.Post("/api/chat", body)
	if err != nil {
		return err
	}
	var resp struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
		SkillCall *struct {
			Skill   string `json:"skill"`
			Confirm bool   `json:"confirm"`
		} `json:"skillCall"`
	}
	if err := json.Unmarshal(data, &resp); err != nil {
		return err
	}
	fmt.Println(resp.Message.Content)
	if resp.SkillCall != nil && resp.SkillCall.Confirm {
		fmt.Printf("\n⚠ Skill %q requires confirmation. Approve? [y/N] ", resp.SkillCall.Skill)
		scanner := bufio.NewScanner(os.Stdin)
		if scanner.Scan() {
			approve := strings.TrimSpace(strings.ToLower(scanner.Text()))
			confirmBody := map[string]any{"approve": approve == "y" || approve == "yes"}
			if chatSessionID != "" {
				confirmBody["session_id"] = chatSessionID
			}
			if approve == "y" || approve == "yes" {
				data, err := client.Post("/api/chat/confirm", confirmBody)
				if err != nil {
					return err
				}
				var confirmResp struct {
					Message struct {
						Content string `json:"content"`
					} `json:"message"`
				}
				if json.Unmarshal(data, &confirmResp) == nil {
					fmt.Println(confirmResp.Message.Content)
				}
			} else {
				client.Post("/api/chat/confirm", confirmBody)
				fmt.Println("Denied.")
			}
		}
	}
	return nil
}

// --- Session ---

func cmdSession(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: kuro-cli session <list|new|delete|history> [args]")
	}
	switch args[0] {
	case "list", "ls":
		data, err := client.Get("/api/chat/sessions")
		if err != nil {
			return err
		}
		var sessions []struct {
			ID      string `json:"id"`
			Title   string `json:"title"`
			Created string `json:"created"`
		}
		json.Unmarshal(data, &sessions)
		if len(sessions) == 0 {
			fmt.Println("No sessions.")
			return nil
		}
		for _, s := range sessions {
			fmt.Printf("%-24s %s\n", s.ID, s.Title)
		}

	case "new":
		data, err := client.Post("/api/chat/sessions", nil)
		if err != nil {
			return err
		}
		var info struct {
			ID    string `json:"id"`
			Title string `json:"title"`
		}
		json.Unmarshal(data, &info)
		fmt.Printf("Created session: %s\n", info.ID)

	case "delete", "rm":
		if len(args) < 2 {
			return fmt.Errorf("usage: kuro-cli session delete <id>")
		}
		_, err := client.Delete("/api/chat/sessions/" + args[1])
		if err != nil {
			return err
		}
		fmt.Printf("Deleted session %q\n", args[1])

	case "history":
		if len(args) < 2 {
			return fmt.Errorf("usage: kuro-cli session history <id>")
		}
		data, err := client.Get("/api/chat/history?session_id=" + args[1])
		if err != nil {
			return err
		}
		var msgs []struct {
			Role    string `json:"role"`
			Content string `json:"content"`
		}
		json.Unmarshal(data, &msgs)
		if len(msgs) == 0 {
			fmt.Println("No messages.")
			return nil
		}
		for _, m := range msgs {
			label := "You"
			if m.Role == "assistant" {
				label = "Kuro"
			}
			fmt.Printf("[%s] %s\n\n", label, m.Content)
		}

	default:
		return fmt.Errorf("unknown session action: %s", args[0])
	}
	return nil
}

// --- Pipeline ---

func cmdPipeline(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: kuro-cli pipeline <list|get|run|create|delete> [args]")
	}
	switch args[0] {
	case "list", "ls":
		data, err := client.Get("/api/pipelines")
		if err != nil {
			return err
		}
		var pipelines []struct {
			Name string `json:"name"`
		}
		json.Unmarshal(data, &pipelines)
		if len(pipelines) == 0 {
			fmt.Println("No pipelines.")
			return nil
		}
		for _, p := range pipelines {
			fmt.Println(p.Name)
		}

	case "get":
		if len(args) < 2 {
			return fmt.Errorf("usage: kuro-cli pipeline get <name>")
		}
		data, err := client.Get("/api/pipelines/" + args[1])
		if err != nil {
			return err
		}
		fmt.Println(cli.PrettyJSON(data))

	case "run":
		if len(args) < 2 {
			return fmt.Errorf("usage: kuro-cli pipeline run <name>")
		}
		data, err := client.Post("/api/pipelines/"+args[1]+"/run", nil)
		if err != nil {
			return err
		}
		fmt.Println(cli.PrettyJSON(data))

	case "create":
		if len(args) < 2 {
			return fmt.Errorf("usage: kuro-cli pipeline create <json-file>")
		}
		content, err := os.ReadFile(args[1])
		if err != nil {
			return err
		}
		var body any
		if err := json.Unmarshal(content, &body); err != nil {
			return fmt.Errorf("invalid JSON: %w", err)
		}
		data, err := client.Post("/api/pipelines", body)
		if err != nil {
			return err
		}
		fmt.Println(cli.PrettyJSON(data))

	case "delete", "rm":
		if len(args) < 2 {
			return fmt.Errorf("usage: kuro-cli pipeline delete <name>")
		}
		_, err := client.Delete("/api/pipelines/" + args[1])
		if err != nil {
			return err
		}
		fmt.Printf("Deleted pipeline %q\n", args[1])

	case "history":
		if len(args) < 2 {
			return fmt.Errorf("usage: kuro-cli pipeline history <name>")
		}
		data, err := client.Get("/api/pipelines/" + args[1] + "/runs")
		if err != nil {
			return err
		}
		fmt.Println(cli.PrettyJSON(data))

	default:
		return fmt.Errorf("unknown pipeline action: %s", args[0])
	}
	return nil
}

// --- Document ---

func cmdDoc(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: kuro-cli doc <list|get|put|delete|search> [args]")
	}
	switch args[0] {
	case "list", "ls":
		data, err := client.Get("/api/documents")
		if err != nil {
			return err
		}
		var docs []struct {
			Name  string `json:"name"`
			IsDir bool   `json:"is_dir"`
		}
		json.Unmarshal(data, &docs)
		if len(docs) == 0 {
			fmt.Println("No documents.")
			return nil
		}
		for _, d := range docs {
			prefix := "  "
			if d.IsDir {
				prefix = "D "
			}
			fmt.Printf("%s%s\n", prefix, d.Name)
		}

	case "get", "cat":
		if len(args) < 2 {
			return fmt.Errorf("usage: kuro-cli doc get <path>")
		}
		data, err := client.Get("/api/documents/" + args[1])
		if err != nil {
			return err
		}
		var doc struct {
			Content string `json:"content"`
		}
		if json.Unmarshal(data, &doc) == nil && doc.Content != "" {
			fmt.Print(doc.Content)
		} else {
			fmt.Print(string(data))
		}

	case "put", "write":
		if len(args) < 3 {
			return fmt.Errorf("usage: kuro-cli doc put <path> <file|->\n  Use '-' to read from stdin")
		}
		var content string
		if args[2] == "-" {
			b, err := os.ReadFile("/dev/stdin")
			if err != nil {
				return err
			}
			content = string(b)
		} else {
			b, err := os.ReadFile(args[2])
			if err != nil {
				return err
			}
			content = string(b)
		}
		_, err := client.Put("/api/documents/"+args[1], map[string]string{"content": content})
		if err != nil {
			return err
		}
		fmt.Printf("Saved %s\n", args[1])

	case "delete", "rm":
		if len(args) < 2 {
			return fmt.Errorf("usage: kuro-cli doc delete <path>")
		}
		_, err := client.Delete("/api/documents/" + args[1])
		if err != nil {
			return err
		}
		fmt.Printf("Deleted %s\n", args[1])

	case "search":
		if len(args) < 2 {
			return fmt.Errorf("usage: kuro-cli doc search <keyword>")
		}
		data, err := client.Get("/api/documents?search=" + args[1])
		if err != nil {
			return err
		}
		fmt.Println(cli.PrettyJSON(data))

	default:
		return fmt.Errorf("unknown doc action: %s", args[0])
	}
	return nil
}

// --- Credential ---

func cmdCred(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: kuro-cli cred <list|add|delete> [args]")
	}
	switch args[0] {
	case "list", "ls":
		data, err := client.Get("/api/credentials")
		if err != nil {
			return err
		}
		var creds []struct {
			Name string `json:"name"`
			Type string `json:"type"`
		}
		json.Unmarshal(data, &creds)
		if len(creds) == 0 {
			fmt.Println("No credentials.")
			return nil
		}
		for _, c := range creds {
			fmt.Printf("%-20s %s\n", c.Name, c.Type)
		}

	case "add":
		if len(args) < 3 {
			return fmt.Errorf("usage: kuro-cli cred add <name> <type> [key=value ...]")
		}
		cred := map[string]any{
			"name": args[1],
			"type": args[2],
		}
		fields := map[string]string{}
		for _, kv := range args[3:] {
			parts := strings.SplitN(kv, "=", 2)
			if len(parts) == 2 {
				fields[parts[0]] = parts[1]
			}
		}
		if len(fields) > 0 {
			cred["fields"] = fields
		}
		_, err := client.Post("/api/credentials", cred)
		if err != nil {
			return err
		}
		fmt.Printf("Created credential %q\n", args[1])

	case "delete", "rm":
		if len(args) < 2 {
			return fmt.Errorf("usage: kuro-cli cred delete <name>")
		}
		_, err := client.Delete("/api/credentials/" + args[1])
		if err != nil {
			return err
		}
		fmt.Printf("Deleted credential %q\n", args[1])

	default:
		return fmt.Errorf("unknown cred action: %s", args[0])
	}
	return nil
}

// --- Skill ---

func cmdSkill(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: kuro-cli skill <list|exec> [args]")
	}
	switch args[0] {
	case "list", "ls":
		data, err := client.Get("/api/skills")
		if err != nil {
			return err
		}
		var skills []struct {
			Name        string `json:"name"`
			Description string `json:"description"`
		}
		json.Unmarshal(data, &skills)
		if len(skills) == 0 {
			fmt.Println("No skills registered.")
			return nil
		}
		for _, s := range skills {
			fmt.Printf("%-20s %s\n", s.Name, s.Description)
		}

	case "exec":
		if len(args) < 2 {
			return fmt.Errorf("usage: kuro-cli skill exec <name> [key=value ...]")
		}
		// Execute skill via chat with a direct invocation
		msg := fmt.Sprintf("Execute skill %s", args[1])
		if len(args) > 2 {
			msg += " with " + strings.Join(args[2:], ", ")
		}
		return chatSend(msg)

	default:
		return fmt.Errorf("unknown skill action: %s", args[0])
	}
	return nil
}

// --- Log ---

func cmdLog(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: kuro-cli log <list|get> [args]")
	}
	switch args[0] {
	case "list", "ls":
		data, err := client.Get("/api/logs")
		if err != nil {
			return err
		}
		var runs []struct {
			ID       string `json:"id"`
			Pipeline string `json:"pipeline_name"`
			Status   string `json:"status"`
		}
		json.Unmarshal(data, &runs)
		if len(runs) == 0 {
			fmt.Println("No runs.")
			return nil
		}
		for _, r := range runs {
			fmt.Printf("%-36s %-20s %s\n", r.ID, r.Pipeline, r.Status)
		}

	case "get":
		if len(args) < 2 {
			return fmt.Errorf("usage: kuro-cli log get <run_id>")
		}
		data, err := client.Get("/api/logs/" + args[1])
		if err != nil {
			return err
		}
		fmt.Println(cli.PrettyJSON(data))

	default:
		return fmt.Errorf("unknown log action: %s", args[0])
	}
	return nil
}

// --- Health ---

func cmdHealth() error {
	data, err := client.Get("/api/health")
	if err != nil {
		return err
	}
	fmt.Println(string(data))
	return nil
}
