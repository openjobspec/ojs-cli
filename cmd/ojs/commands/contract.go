package commands

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/openjobspec/ojs-cli/internal/output"
)

// ContractCommand handles the "contract" subcommand.
type ContractCommand struct{}

// Run executes the contract command.
func (c *ContractCommand) Run(args []string) error {
	if len(args) == 0 {
		return c.printUsage()
	}

	switch args[0] {
	case "test":
		return c.runTest(args[1:])
	case "validate":
		return c.runValidate(args[1:])
	case "init":
		return c.runInit(args[1:])
	default:
		return fmt.Errorf("unknown contract subcommand: %s", args[0])
	}
}

func (c *ContractCommand) printUsage() error {
	fmt.Println(`Usage: ojs contract <command>

Commands:
  test       Validate contracts against schema registry
  validate   Check contract file syntax
  init       Generate a template contract file

Examples:
  ojs contract test --contracts contracts.json
  ojs contract test --contracts contracts.json --registry http://localhost:8080
  ojs contract validate --contracts contracts.json
  ojs contract init --service my-service --role consumer > contracts.json`)
	return nil
}

// --- contract test ---

func (c *ContractCommand) runTest(args []string) error {
	contractFile := ""
	registryURL := ""

	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--contracts", "-c":
			if i+1 < len(args) {
				contractFile = args[i+1]
				i++
			}
		case "--registry", "-r":
			if i+1 < len(args) {
				registryURL = args[i+1]
				i++
			}
		}
	}

	if contractFile == "" {
		return fmt.Errorf("--contracts flag is required (path to contracts JSON file)")
	}

	data, err := os.ReadFile(contractFile)
	if err != nil {
		return fmt.Errorf("reading contracts file: %w", err)
	}

	var contracts []ContractDef
	if err := json.Unmarshal(data, &contracts); err != nil {
		return fmt.Errorf("parsing contracts file: %w", err)
	}

	// Run contract validation
	results := validateContracts(contracts, registryURL)

	// Output results
	if output.Format == "json" {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(results); err != nil {
			return err
		}
		if results.Failed > 0 {
			return fmt.Errorf("%d contract(s) failed validation", results.Failed)
		}
		return nil
	}

	// Table output
	passed, failed := 0, 0
	for _, r := range results.Results {
		status := "✅ PASS"
		if !r.Passed {
			status = "❌ FAIL"
			failed++
		} else {
			passed++
		}
		fmt.Printf("  %s %s/%s (%s)\n", status, r.Service, r.JobType, r.Role)
		for _, e := range r.Errors {
			fmt.Printf("    ERROR: %s\n", e)
		}
		for _, w := range r.Warnings {
			fmt.Printf("    WARN:  %s\n", w)
		}
	}

	fmt.Printf("\nContract Tests: %d passed, %d failed\n", passed, failed)

	if failed > 0 {
		return fmt.Errorf("%d contract(s) failed validation", failed)
	}
	return nil
}

// --- contract validate ---

func (c *ContractCommand) runValidate(args []string) error {
	contractFile := ""
	for i := 0; i < len(args); i++ {
		if args[i] == "--contracts" || args[i] == "-c" {
			if i+1 < len(args) {
				contractFile = args[i+1]
				i++
			}
		}
	}

	if contractFile == "" {
		return fmt.Errorf("--contracts flag is required")
	}

	data, err := os.ReadFile(contractFile)
	if err != nil {
		return fmt.Errorf("reading contracts file: %w", err)
	}

	var contracts []ContractDef
	if err := json.Unmarshal(data, &contracts); err != nil {
		return fmt.Errorf("invalid JSON: %w", err)
	}

	// Validate structure
	var errors []string
	for i, c := range contracts {
		if c.Service == "" {
			errors = append(errors, fmt.Sprintf("contract[%d]: missing service name", i))
		}
		if c.JobType == "" {
			errors = append(errors, fmt.Sprintf("contract[%d]: missing job_type", i))
		}
		if c.Role != "producer" && c.Role != "consumer" {
			errors = append(errors, fmt.Sprintf("contract[%d]: role must be 'producer' or 'consumer', got %q", i, c.Role))
		}
	}

	if len(errors) > 0 {
		for _, e := range errors {
			fmt.Printf("  ❌ %s\n", e)
		}
		return fmt.Errorf("%d validation error(s)", len(errors))
	}

	fmt.Printf("✅ %d contract(s) are syntactically valid\n", len(contracts))
	return nil
}

// --- contract init ---

func (c *ContractCommand) runInit(args []string) error {
	service := "my-service"
	role := "consumer"

	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--service":
			if i+1 < len(args) {
				service = args[i+1]
				i++
			}
		case "--role":
			if i+1 < len(args) {
				role = args[i+1]
				i++
			}
		}
	}

	template := []ContractDef{
		{
			Service: service,
			Role:    role,
			JobType: "email.send",
			Args: []ContractArg{
				{Name: "to", Type: "string", Required: true},
				{Name: "subject", Type: "string", Required: true},
				{Name: "body", Type: "string", Required: false},
			},
		},
	}

	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(template)
}

// --- Types ---

// ContractDef is the CLI-facing contract definition (mirrors registry.Contract).
type ContractDef struct {
	Service string        `json:"service"`
	Role    string        `json:"role"`
	JobType string        `json:"job_type"`
	Version string        `json:"version,omitempty"`
	Args    []ContractArg `json:"args"`
}

// ContractArg defines a single argument.
type ContractArg struct {
	Name     string `json:"name"`
	Type     string `json:"type"`
	Required bool   `json:"required"`
}

// ContractTestResult is a single contract validation result.
type ContractTestResult struct {
	Service  string   `json:"service"`
	Role     string   `json:"role"`
	JobType  string   `json:"job_type"`
	Passed   bool     `json:"passed"`
	Errors   []string `json:"errors,omitempty"`
	Warnings []string `json:"warnings,omitempty"`
}

// ContractTestSuite is the complete validation output.
type ContractTestSuite struct {
	Results []ContractTestResult `json:"results"`
	Passed  int                  `json:"passed"`
	Failed  int                  `json:"failed"`
}

// validateContracts runs pairwise validation between producer/consumer contracts.
func validateContracts(contracts []ContractDef, registryURL string) *ContractTestSuite {
	suite := &ContractTestSuite{}

	// Group by job type
	producers := make(map[string][]ContractDef)
	consumers := make(map[string][]ContractDef)

	for _, c := range contracts {
		switch c.Role {
		case "producer":
			producers[c.JobType] = append(producers[c.JobType], c)
		case "consumer":
			consumers[c.JobType] = append(consumers[c.JobType], c)
		}
	}

	// Cross-validate
	for jobType, prods := range producers {
		cons := consumers[jobType]
		for _, p := range prods {
			for _, c := range cons {
				result := crossValidate(p, c)
				suite.Results = append(suite.Results, result)
				if result.Passed {
					suite.Passed++
				} else {
					suite.Failed++
				}
			}
		}
	}

	// Validate contracts without counterpart
	for _, contracts := range producers {
		for _, c := range contracts {
			if _, hasCons := consumers[c.JobType]; !hasCons {
				suite.Results = append(suite.Results, ContractTestResult{
					Service:  c.Service,
					Role:     c.Role,
					JobType:  c.JobType,
					Passed:   true,
					Warnings: []string{"no consumer contract found for this job type"},
				})
				suite.Passed++
			}
		}
	}
	for _, contracts := range consumers {
		for _, c := range contracts {
			if _, hasProds := producers[c.JobType]; !hasProds {
				suite.Results = append(suite.Results, ContractTestResult{
					Service:  c.Service,
					Role:     c.Role,
					JobType:  c.JobType,
					Passed:   true,
					Warnings: []string{"no producer contract found for this job type"},
				})
				suite.Passed++
			}
		}
	}

	return suite
}

func crossValidate(producer, consumer ContractDef) ContractTestResult {
	result := ContractTestResult{
		Service: fmt.Sprintf("%s↔%s", producer.Service, consumer.Service),
		Role:    "cross",
		JobType: producer.JobType,
		Passed:  true,
	}

	prodArgs := make(map[string]ContractArg)
	for _, a := range producer.Args {
		prodArgs[a.Name] = a
	}

	for _, expected := range consumer.Args {
		prodArg, exists := prodArgs[expected.Name]
		if !exists {
			if expected.Required {
				result.Errors = append(result.Errors,
					fmt.Sprintf("consumer %s expects required arg %q but producer %s doesn't provide it",
						consumer.Service, expected.Name, producer.Service))
				result.Passed = false
			}
			continue
		}

		if expected.Type != "" && prodArg.Type != expected.Type {
			result.Errors = append(result.Errors,
				fmt.Sprintf("arg %q type mismatch: producer=%s, consumer=%s",
					expected.Name, prodArg.Type, expected.Type))
			result.Passed = false
		}
	}

	return result
}

// RunContractCommand is the entry point called from main.go.
func RunContractCommand(args []string) error {
	cmd := &ContractCommand{}
	return cmd.Run(args)
}

// Helper to split "x.y" → find if it contains separators
func splitDot(s string) []string {
	return strings.Split(s, ".")
}
