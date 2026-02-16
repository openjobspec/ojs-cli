package commands

import (
	"encoding/json"
	"flag"
	"fmt"

	"github.com/openjobspec/ojs-cli/internal/client"
	"github.com/openjobspec/ojs-cli/internal/output"
)

// System manages system-level operations.
func System(c *client.Client, args []string) error {
	if len(args) == 0 {
		return printSystemUsage()
	}

	switch args[0] {
	case "maintenance":
		return systemMaintenance(c, args[1:])
	case "config":
		return systemConfig(c)
	default:
		return printSystemUsage()
	}
}

func systemMaintenance(c *client.Client, args []string) error {
	fs := flag.NewFlagSet("system maintenance", flag.ExitOnError)
	enable := fs.Bool("enable", false, "Enable maintenance mode")
	disable := fs.Bool("disable", false, "Disable maintenance mode")
	reason := fs.String("reason", "", "Reason for maintenance")
	fs.Parse(args)

	if !*enable && !*disable {
		// Show current status
		data, _, err := c.Get("/admin/maintenance")
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
		enabled := str(resp["enabled"])
		if enabled == "true" {
			fmt.Printf("Maintenance mode: ENABLED\n")
			if resp["reason"] != nil {
				fmt.Printf("Reason: %s\n", str(resp["reason"]))
			}
			if resp["started_at"] != nil {
				fmt.Printf("Since: %s\n", str(resp["started_at"]))
			}
		} else {
			fmt.Println("Maintenance mode: DISABLED")
		}
		return nil
	}

	if *enable && *disable {
		return fmt.Errorf("cannot use both --enable and --disable")
	}

	body := map[string]any{
		"enabled": *enable,
	}
	if *reason != "" {
		body["reason"] = *reason
	}

	data, _, err := c.Post("/admin/maintenance", body)
	if err != nil {
		return err
	}

	if output.Format == "json" {
		var result any
		json.Unmarshal(data, &result)
		return output.JSON(result)
	}

	if *enable {
		msg := "Maintenance mode enabled"
		if *reason != "" {
			msg += fmt.Sprintf(" (reason: %s)", *reason)
		}
		output.Success(msg)
	} else {
		output.Success("Maintenance mode disabled")
	}
	return nil
}

func systemConfig(c *client.Client) error {
	data, _, err := c.Get("/admin/config")
	if err != nil {
		return err
	}

	if output.Format == "json" {
		var result any
		json.Unmarshal(data, &result)
		return output.JSON(result)
	}

	var result any
	json.Unmarshal(data, &result)
	return output.JSON(result)
}

func printSystemUsage() error {
	return fmt.Errorf("subcommand required\n\nUsage: ojs system <subcommand>\n\n" +
		"Subcommands:\n" +
		"  maintenance  Manage maintenance mode (--enable/--disable)\n" +
		"  config       View system configuration")
}
