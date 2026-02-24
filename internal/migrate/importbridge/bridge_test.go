package importbridge

import (
	"strings"
	"testing"
)

const sampleTemporalGo = `
package workflows

import (
	"go.temporal.io/sdk/workflow"
)

func OrderWorkflow(ctx workflow.Context, orderID string) error {
	var result string

	err := workflow.ExecuteActivity(ctx, ValidateOrder, orderID).Get(ctx, &result)
	if err != nil {
		return err
	}

	err = workflow.ExecuteActivity(ctx, ProcessPayment, orderID).Get(ctx, &result)
	if err != nil {
		return err
	}

	err = workflow.ExecuteActivity(ctx, FulfillOrder, orderID).Get(ctx, &result)
	if err != nil {
		return err
	}

	return nil
}
`

const sampleTemporalParallel = `
package workflows

import (
	"go.temporal.io/sdk/workflow"
)

func ParallelWorkflow(ctx workflow.Context) error {
	workflow.Go(ctx, func(gCtx workflow.Context) {
		workflow.ExecuteActivity(gCtx, TaskA)
	})
	workflow.Go(ctx, func(gCtx workflow.Context) {
		workflow.ExecuteActivity(gCtx, TaskB)
	})
	return nil
}
`

const sampleAirflowDAG = `
from airflow import DAG
from airflow.operators.python import PythonOperator
from airflow.operators.bash import BashOperator
from datetime import datetime

with DAG(
    'etl_pipeline',
    start_date=datetime(2024, 1, 1),
    schedule='@daily',
) as dag:
    extract = PythonOperator(
        task_id='extract_data',
        python_callable=extract_fn,
    )

    transform = PythonOperator(
        task_id='transform_data',
        python_callable=transform_fn,
    )

    load = BashOperator(
        task_id='load_data',
        bash_command='load.sh',
    )

    extract >> transform >> load
`

func TestParseTemporalGo(t *testing.T) {
	workflows, warnings := ParseTemporalGo(sampleTemporalGo)
	if len(workflows) != 1 {
		t.Fatalf("expected 1 workflow, got %d", len(workflows))
	}
	wf := workflows[0]
	if wf.Name != "OrderWorkflow" {
		t.Errorf("expected OrderWorkflow, got %s", wf.Name)
	}
	if wf.Source != "temporal" {
		t.Errorf("expected temporal, got %s", wf.Source)
	}
	if len(wf.Steps) != 3 {
		t.Errorf("expected 3 activities, got %d", len(wf.Steps))
	}
	_ = warnings
}

func TestParseTemporalParallel(t *testing.T) {
	workflows, warnings := ParseTemporalGo(sampleTemporalParallel)
	if len(workflows) != 1 {
		t.Fatal("expected 1 workflow")
	}
	if !workflows[0].Parallel {
		t.Error("expected parallel flag for goroutine-based workflow")
	}
	if len(warnings) == 0 {
		t.Error("expected warnings for parallel pattern")
	}
}

func TestParseAirflowDAG(t *testing.T) {
	workflows, warnings := ParseAirflowDAG(sampleAirflowDAG)
	if len(workflows) != 1 {
		t.Fatalf("expected 1 workflow, got %d", len(workflows))
	}
	wf := workflows[0]
	if wf.Name != "etl_pipeline" {
		t.Errorf("expected etl_pipeline, got %s", wf.Name)
	}
	if wf.Source != "airflow" {
		t.Errorf("expected airflow, got %s", wf.Source)
	}
	if len(wf.Steps) != 3 {
		t.Errorf("expected 3 tasks, got %d", len(wf.Steps))
	}
	_ = warnings
}

func TestToOJSYAML(t *testing.T) {
	workflows, _ := ParseTemporalGo(sampleTemporalGo)
	yaml := ToOJSYAML(workflows)

	if !strings.Contains(yaml, "version: \"1.0\"") {
		t.Error("expected version in YAML")
	}
	if !strings.Contains(yaml, "validate.order") {
		t.Error("expected validate.order job type")
	}
	if !strings.Contains(yaml, "process.payment") {
		t.Error("expected process.payment job type")
	}
	if !strings.Contains(yaml, "chain") {
		t.Error("expected chain workflow type")
	}
}

func TestToOJSYAMLParallel(t *testing.T) {
	workflows, _ := ParseTemporalGo(sampleTemporalParallel)
	yaml := ToOJSYAML(workflows)

	if !strings.Contains(yaml, "group") {
		t.Error("expected group workflow type for parallel")
	}
}

func TestToOJSJobType(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"ValidateOrder", "validate.order"},
		{"ProcessPayment", "process.payment"},
		{"extract_data", "extract.data"},
		{"send_email", "send.email"},
	}
	for _, tt := range tests {
		got := toOJSJobType(tt.input)
		if got != tt.expected {
			t.Errorf("toOJSJobType(%q) = %q, want %q", tt.input, got, tt.expected)
		}
	}
}

func TestAirflowYAMLGeneration(t *testing.T) {
	workflows, _ := ParseAirflowDAG(sampleAirflowDAG)
	yaml := ToOJSYAML(workflows)

	if !strings.Contains(yaml, "extract.data") {
		t.Error("expected extract.data in YAML")
	}
	if !strings.Contains(yaml, "Migrated from airflow") {
		t.Error("expected airflow migration note")
	}
}

func TestEmptySource(t *testing.T) {
	workflows, _ := ParseTemporalGo("")
	if len(workflows) != 0 {
		t.Error("expected 0 workflows from empty source")
	}

	workflows, _ = ParseAirflowDAG("")
	if len(workflows) != 0 {
		t.Error("expected 0 workflows from empty source")
	}
}
