package commands

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestContractValidate(t *testing.T) {
	// Create temp contracts file
	contracts := []ContractDef{
		{Service: "api", Role: "producer", JobType: "email.send", Args: []ContractArg{{Name: "to", Type: "string", Required: true}}},
		{Service: "worker", Role: "consumer", JobType: "email.send", Args: []ContractArg{{Name: "to", Type: "string", Required: true}}},
	}
	data, _ := json.Marshal(contracts)
	tmpFile := filepath.Join(t.TempDir(), "contracts.json")
	os.WriteFile(tmpFile, data, 0644)

	cmd := &ContractCommand{}
	err := cmd.Run([]string{"validate", "--contracts", tmpFile})
	if err != nil {
		t.Errorf("expected valid contracts: %v", err)
	}
}

func TestContractValidateInvalid(t *testing.T) {
	contracts := []ContractDef{
		{Service: "", Role: "invalid", JobType: ""},
	}
	data, _ := json.Marshal(contracts)
	tmpFile := filepath.Join(t.TempDir(), "bad.json")
	os.WriteFile(tmpFile, data, 0644)

	cmd := &ContractCommand{}
	err := cmd.Run([]string{"validate", "--contracts", tmpFile})
	if err == nil {
		t.Error("expected validation error")
	}
}

func TestContractTestPassing(t *testing.T) {
	contracts := []ContractDef{
		{Service: "api", Role: "producer", JobType: "order.process", Args: []ContractArg{
			{Name: "order_id", Type: "string", Required: true},
			{Name: "amount", Type: "float", Required: true},
		}},
		{Service: "worker", Role: "consumer", JobType: "order.process", Args: []ContractArg{
			{Name: "order_id", Type: "string", Required: true},
		}},
	}
	data, _ := json.Marshal(contracts)
	tmpFile := filepath.Join(t.TempDir(), "contracts.json")
	os.WriteFile(tmpFile, data, 0644)

	cmd := &ContractCommand{}
	err := cmd.Run([]string{"test", "--contracts", tmpFile})
	if err != nil {
		t.Errorf("expected passing test: %v", err)
	}
}

func TestContractTestFailing(t *testing.T) {
	contracts := []ContractDef{
		{Service: "api", Role: "producer", JobType: "task.run", Args: []ContractArg{
			{Name: "task_id", Type: "string", Required: true},
		}},
		{Service: "worker", Role: "consumer", JobType: "task.run", Args: []ContractArg{
			{Name: "task_id", Type: "string", Required: true},
			{Name: "priority", Type: "int", Required: true}, // producer doesn't provide this
		}},
	}
	data, _ := json.Marshal(contracts)
	tmpFile := filepath.Join(t.TempDir(), "contracts.json")
	os.WriteFile(tmpFile, data, 0644)

	cmd := &ContractCommand{}
	err := cmd.Run([]string{"test", "--contracts", tmpFile})
	if err == nil {
		t.Error("expected failing test for missing required arg")
	}
}

func TestContractTestMissingFile(t *testing.T) {
	cmd := &ContractCommand{}
	err := cmd.Run([]string{"test", "--contracts", "/nonexistent/file.json"})
	if err == nil {
		t.Error("expected error for missing file")
	}
}

func TestContractTestNoFlag(t *testing.T) {
	cmd := &ContractCommand{}
	err := cmd.Run([]string{"test"})
	if err == nil {
		t.Error("expected error for missing --contracts flag")
	}
}

func TestCrossValidatePass(t *testing.T) {
	p := ContractDef{Service: "api", Role: "producer", Args: []ContractArg{
		{Name: "id", Type: "string", Required: true},
		{Name: "data", Type: "object", Required: true},
	}}
	c := ContractDef{Service: "worker", Role: "consumer", Args: []ContractArg{
		{Name: "id", Type: "string", Required: true},
	}}

	result := crossValidate(p, c)
	if !result.Passed {
		t.Errorf("expected pass: %v", result.Errors)
	}
}

func TestCrossValidateTypeMismatch(t *testing.T) {
	p := ContractDef{Service: "api", Role: "producer", Args: []ContractArg{
		{Name: "count", Type: "string", Required: true},
	}}
	c := ContractDef{Service: "worker", Role: "consumer", Args: []ContractArg{
		{Name: "count", Type: "int", Required: true},
	}}

	result := crossValidate(p, c)
	if result.Passed {
		t.Error("expected failure for type mismatch")
	}
}

func TestContractUsage(t *testing.T) {
	cmd := &ContractCommand{}
	err := cmd.Run([]string{})
	if err != nil {
		t.Errorf("usage should not error: %v", err)
	}
}
