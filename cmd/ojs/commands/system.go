package commands

import (
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
	fs := flag.NewFlagSet("system maintenance", flag.ContinueOnError)
	enable := fs.Bool("enable", false, "Enable maintenance mode")
	disable := fs.Bool("disable", false, "Disable maintenance mode")
	reason := fs.String("reason", "", "Reason for maintenance")
	if err := fs.Parse(args); err != nil {
		return fmt.Errorf("parse flags: %w", err)
	}

	if !*enable && !*disable {
		return showMaintenance(c)
	}

	if *enable && *disable {
		return fmt.Errorf("cannot use both --enable and --disable")
	}
	return setMaintenance(c, *enable, *reason)
}

func showMaintenance(c *client.Client) error {
	data, _, err := c.Get("/admin/maintenance")
	if err != nil {
		return err
	}
	if output.Format == "json" {
		return printJSONResponse(data)
	}
	var response map[string]any
	if err := decodeResponse(data, &response); err != nil {
		return err
	}
	if str(response["enabled"]) != "true" {
		fmt.Println("Maintenance mode: DISABLED")
		return nil
	}
	fmt.Println("Maintenance mode: ENABLED")
	if response["reason"] != nil {
		fmt.Printf("Reason: %s\n", str(response["reason"]))
	}
	if response["started_at"] != nil {
		fmt.Printf("Since: %s\n", str(response["started_at"]))
	}
	return nil
}

func setMaintenance(c *client.Client, enable bool, reason string) error {
	body := map[string]any{
		"enabled": enable,
	}
	if reason != "" {
		body["reason"] = reason
	}

	data, _, err := c.Post("/admin/maintenance", body)
	if err != nil {
		return err
	}

	if output.Format == "json" {
		return printJSONResponse(data)
	}

	if enable {
		msg := "Maintenance mode enabled"
		if reason != "" {
			msg += fmt.Sprintf(" (reason: %s)", reason)
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

	return printJSONResponse(data)
}

func printSystemUsage() error {
	return fmt.Errorf("subcommand required\n\nUsage: ojs system <subcommand>\n\n" +
		"Subcommands:\n" +
		"  maintenance  Manage maintenance mode (--enable/--disable)\n" +
		"  config       View system configuration")
}
