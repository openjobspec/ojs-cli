package commands

import (
	"flag"
	"fmt"

	"github.com/openjobspec/ojs-cli/internal/client"
	"github.com/openjobspec/ojs-cli/internal/output"
)

// RateLimits manages rate limit inspection and overrides.
func RateLimits(c *client.Client, args []string) error {
	fs := flag.NewFlagSet("rate-limits", flag.ContinueOnError)
	inspect := fs.String("inspect", "", "Inspect rate limit by key")
	override := fs.String("override", "", "Override rate limit by key")
	concurrency := fs.Int("concurrency", 0, "Concurrency limit (for override)")
	clear := fs.Bool("clear", false, "Clear rate limit override")
	if err := fs.Parse(args); err != nil {
		return fmt.Errorf("parse flags: %w", err)
	}

	if *override != "" {
		if *clear {
			_, _, err := c.Delete("/rate-limits/" + *override + "/override")
			if err != nil {
				return err
			}
			output.Success("Rate limit override cleared for %q", *override)
			return nil
		}
		if *concurrency <= 0 {
			return fmt.Errorf("--concurrency is required for override\n\n" +
				"Usage: ojs rate-limits --override <key> --concurrency <n>\n" +
				"       ojs rate-limits --override <key> --clear")
		}
		body := map[string]any{
			"concurrency": *concurrency,
		}
		data, _, err := c.Put("/rate-limits/"+*override+"/override", body)
		if err != nil {
			return err
		}
		if output.Format == "json" {
			return printJSONResponse(data)
		}
		output.Success("Rate limit override set for %q (concurrency=%d)", *override, *concurrency)
		return nil
	}

	if *inspect != "" {
		data, _, err := c.Get("/rate-limits/" + *inspect)
		if err != nil {
			return err
		}
		if output.Format == "json" {
			return printJSONResponse(data)
		}
		var rl map[string]any
		if err := decodeResponse(data, &rl); err != nil {
			return err
		}
		headers := []string{"FIELD", "VALUE"}
		rows := [][]string{
			{"Key", str(rl["key"])},
			{"Concurrency", str(rl["concurrency"])},
			{"Active", str(rl["active"])},
			{"Available", str(rl["available"])},
		}
		if rl["override"] != nil {
			rows = append(rows, []string{"Override", str(rl["override"])})
		}
		output.Table(headers, rows)
		return nil
	}

	return listRateLimits(c)
}

func listRateLimits(c *client.Client) error {
	data, _, err := c.Get("/rate-limits")
	if err != nil {
		return err
	}

	if output.Format == "json" {
		return printJSONResponse(data)
	}

	var resp struct {
		RateLimits []struct {
			Key         string `json:"key"`
			Concurrency int    `json:"concurrency"`
			Active      int    `json:"active"`
			Available   int    `json:"available"`
		} `json:"rate_limits"`
	}
	if err := decodeResponse(data, &resp); err != nil {
		return err
	}

	if len(resp.RateLimits) == 0 {
		fmt.Println("No rate limits configured.")
		return nil
	}

	headers := []string{"KEY", "CONCURRENCY", "ACTIVE", "AVAILABLE"}
	rows := make([][]string, 0, len(resp.RateLimits))
	for _, rl := range resp.RateLimits {
		rows = append(rows, []string{
			rl.Key,
			fmt.Sprintf("%d", rl.Concurrency),
			fmt.Sprintf("%d", rl.Active),
			fmt.Sprintf("%d", rl.Available),
		})
	}
	output.Table(headers, rows)
	return nil
}
