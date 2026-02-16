package commands

import (
	"encoding/json"
	"flag"
	"fmt"

	"github.com/openjobspec/ojs-cli/internal/client"
	"github.com/openjobspec/ojs-cli/internal/output"
)

// Webhooks manages webhook subscriptions.
func Webhooks(c *client.Client, args []string) error {
	if len(args) == 0 {
		return printWebhooksUsage()
	}

	switch args[0] {
	case "create":
		return webhookCreate(c, args[1:])
	case "list":
		return webhookList(c, args[1:])
	case "get":
		return webhookGet(c, args[1:])
	case "delete":
		return webhookDelete(c, args[1:])
	case "test":
		return webhookTest(c, args[1:])
	case "rotate-secret":
		return webhookRotateSecret(c, args[1:])
	default:
		return printWebhooksUsage()
	}
}

func webhookCreate(c *client.Client, args []string) error {
	fs := flag.NewFlagSet("webhooks create", flag.ExitOnError)
	url := fs.String("url", "", "Webhook endpoint URL (required)")
	events := fs.String("events", "", "Comma-separated event types to subscribe to (required)")
	secret := fs.String("secret", "", "Shared secret for HMAC signature verification")
	fs.Parse(args)

	if *url == "" || *events == "" {
		return fmt.Errorf("--url and --events are required\n\n" +
			"Usage: ojs webhooks create --url <url> --events <event1,event2>\n\n" +
			"Example:\n" +
			"  ojs webhooks create --url https://example.com/hooks --events job.completed,job.failed")
	}

	body := map[string]any{
		"url":    *url,
		"events": splitIDs(*events),
	}
	if *secret != "" {
		body["secret"] = *secret
	}

	data, _, err := c.Post("/webhooks/subscriptions", body)
	if err != nil {
		return err
	}

	if output.Format == "json" {
		var result any
		json.Unmarshal(data, &result)
		return output.JSON(result)
	}

	var resp map[string]any
	json.Unmarshal(data, &resp)
	output.Success("Webhook subscription created: %s (url=%s)", str(resp["id"]), *url)
	return nil
}

func webhookList(c *client.Client, args []string) error {
	fs := flag.NewFlagSet("webhooks list", flag.ExitOnError)
	limit := fs.Int("limit", 25, "Max results to return")
	fs.Parse(args)

	data, _, err := c.Get(fmt.Sprintf("/webhooks/subscriptions?limit=%d", *limit))
	if err != nil {
		return err
	}

	if output.Format == "json" {
		var result any
		json.Unmarshal(data, &result)
		return output.JSON(result)
	}

	var resp struct {
		Subscriptions []struct {
			ID        string   `json:"id"`
			URL       string   `json:"url"`
			Events    []string `json:"events"`
			Active    bool     `json:"active"`
			CreatedAt string   `json:"created_at"`
		} `json:"subscriptions"`
		Total int `json:"total"`
	}
	json.Unmarshal(data, &resp)

	fmt.Printf("Webhook subscriptions: %d total\n\n", resp.Total)

	if len(resp.Subscriptions) == 0 {
		fmt.Println("No webhook subscriptions.")
		return nil
	}

	headers := []string{"ID", "URL", "EVENTS", "ACTIVE", "CREATED"}
	rows := make([][]string, 0, len(resp.Subscriptions))
	for _, s := range resp.Subscriptions {
		active := "✓"
		if !s.Active {
			active = "✗"
		}
		eventsStr := ""
		for i, e := range s.Events {
			if i > 0 {
				eventsStr += ", "
			}
			eventsStr += e
		}
		rows = append(rows, []string{s.ID, s.URL, eventsStr, active, s.CreatedAt})
	}
	output.Table(headers, rows)
	return nil
}

func webhookGet(c *client.Client, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("subscription ID required\n\nUsage: ojs webhooks get <subscription-id>")
	}

	subID := args[0]
	data, _, err := c.Get("/webhooks/subscriptions/" + subID)
	if err != nil {
		return err
	}

	if output.Format == "json" {
		var result any
		json.Unmarshal(data, &result)
		return output.JSON(result)
	}

	var sub struct {
		ID             string   `json:"id"`
		URL            string   `json:"url"`
		Events         []string `json:"events"`
		Active         bool     `json:"active"`
		CreatedAt      string   `json:"created_at"`
		LastDeliveryAt string   `json:"last_delivery_at"`
		SuccessCount   int      `json:"success_count"`
		FailureCount   int      `json:"failure_count"`
	}
	json.Unmarshal(data, &sub)

	active := "true"
	if !sub.Active {
		active = "false"
	}
	eventsStr := ""
	for i, e := range sub.Events {
		if i > 0 {
			eventsStr += ", "
		}
		eventsStr += e
	}
	lastDelivery := sub.LastDeliveryAt
	if lastDelivery == "" {
		lastDelivery = "-"
	}

	headers := []string{"FIELD", "VALUE"}
	rows := [][]string{
		{"ID", sub.ID},
		{"URL", sub.URL},
		{"Events", eventsStr},
		{"Active", active},
		{"Created", sub.CreatedAt},
		{"Last Delivery", lastDelivery},
		{"Successes", fmt.Sprintf("%d", sub.SuccessCount)},
		{"Failures", fmt.Sprintf("%d", sub.FailureCount)},
	}
	output.Table(headers, rows)
	return nil
}

func webhookDelete(c *client.Client, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("subscription ID required\n\nUsage: ojs webhooks delete <subscription-id>")
	}

	subID := args[0]
	_, _, err := c.Delete("/webhooks/subscriptions/" + subID)
	if err != nil {
		return err
	}

	output.Success("Webhook subscription %s deleted", subID)
	return nil
}

func webhookTest(c *client.Client, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("subscription ID required\n\nUsage: ojs webhooks test <subscription-id>")
	}

	subID := args[0]
	data, _, err := c.Post("/webhooks/subscriptions/"+subID+"/test", nil)
	if err != nil {
		return err
	}

	if output.Format == "json" {
		var result any
		json.Unmarshal(data, &result)
		return output.JSON(result)
	}

	var resp struct {
		StatusCode int  `json:"status_code"`
		Success    bool `json:"success"`
	}
	json.Unmarshal(data, &resp)

	if resp.Success {
		output.Success("Test webhook delivered successfully (status=%d)", resp.StatusCode)
	} else {
		output.Warn("Test webhook delivery failed (status=%d)", resp.StatusCode)
	}
	return nil
}

func webhookRotateSecret(c *client.Client, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("subscription ID required\n\nUsage: ojs webhooks rotate-secret <subscription-id>")
	}

	subID := args[0]
	data, _, err := c.Post("/webhooks/subscriptions/"+subID+"/rotate-secret", nil)
	if err != nil {
		return err
	}

	if output.Format == "json" {
		var result any
		json.Unmarshal(data, &result)
		return output.JSON(result)
	}

	var resp map[string]any
	json.Unmarshal(data, &resp)
	output.Success("Webhook secret rotated for subscription %s", subID)
	if resp["new_secret"] != nil {
		fmt.Printf("New secret: %s\n", str(resp["new_secret"]))
	}
	return nil
}

func printWebhooksUsage() error {
	return fmt.Errorf("subcommand required\n\nUsage: ojs webhooks <subcommand>\n\n" +
		"Subcommands:\n" +
		"  create         Create a webhook subscription\n" +
		"  list           List webhook subscriptions\n" +
		"  get            Get webhook subscription details\n" +
		"  delete         Delete a webhook subscription\n" +
		"  test           Send a test webhook\n" +
		"  rotate-secret  Rotate the webhook signing secret")
}
