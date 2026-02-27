package migrate

import (
	"testing"
)

func TestAnalyzeSidekiq(t *testing.T) {
	files := map[string]string{
		"app/workers/email_worker.rb": `class EmailWorker
  include Sidekiq::Worker
  sidekiq_options queue: :mailer, retry: 5

  def perform(to, subject, body)
    UserMailer.send_email(to, subject, body)
  end
end`,
		"app/workers/report_worker.rb": `class ReportGenerator
  include Sidekiq::Worker
  sidekiq_options queue: :reports

  def perform(report_id)
    Report.find(report_id).generate!
  end
end`,
	}

	result := AnalyzeSource(FrameworkSidekiq, files)

	if result.Framework != FrameworkSidekiq {
		t.Errorf("expected sidekiq, got %s", result.Framework)
	}
	if len(result.Jobs) != 2 {
		t.Fatalf("expected 2 jobs, got %d", len(result.Jobs))
	}
	if result.TotalFiles != 2 {
		t.Errorf("expected 2 files, got %d", result.TotalFiles)
	}

	// Verify queue extraction
	found := false
	for _, job := range result.Jobs {
		if job.Name == "EmailWorker" && job.Queue == "mailer" {
			found = true
		}
	}
	if !found {
		t.Error("expected EmailWorker with queue=mailer")
	}
}

func TestAnalyzeBullMQ(t *testing.T) {
	files := map[string]string{
		"src/workers.ts": `
const emailQueue = new Queue('email-queue');
const worker = new Worker('email-queue', async (job) => {
  await sendEmail(job.data.to, job.data.subject);
});
await emailQueue.add('send-welcome', { to: 'user@example.com' });
`,
	}

	result := AnalyzeSource(FrameworkBullMQ, files)

	if len(result.Jobs) < 1 {
		t.Fatalf("expected at least 1 job, got %d", len(result.Jobs))
	}
}

func TestAnalyzeCelery(t *testing.T) {
	files := map[string]string{
		"tasks.py": `
from celery import shared_task

@shared_task(queue='emails')
def send_notification(user_id, message):
    user = User.objects.get(id=user_id)
    user.notify(message)

@app.task
def process_payment(order_id):
    order = Order.objects.get(id=order_id)
    order.charge()
`,
	}

	result := AnalyzeSource(FrameworkCelery, files)

	if len(result.Jobs) != 2 {
		t.Fatalf("expected 2 jobs, got %d", len(result.Jobs))
	}

	found := false
	for _, job := range result.Jobs {
		if job.Name == "send_notification" && job.Queue == "emails" {
			found = true
		}
	}
	if !found {
		t.Error("expected send_notification with queue=emails")
	}
}

func TestAnalyzeEmptyFiles(t *testing.T) {
	result := AnalyzeSource(FrameworkSidekiq, map[string]string{
		"empty.rb": "# no workers here",
	})

	if len(result.Jobs) != 0 {
		t.Errorf("expected 0 jobs, got %d", len(result.Jobs))
	}
	if len(result.Warnings) == 0 {
		t.Error("expected warning for no jobs found")
	}
}

func TestGenerateOJSCodeGo(t *testing.T) {
	job := JobDefinition{
		Name:      "EmailWorker",
		Queue:     "mailer",
		Framework: FrameworkSidekiq,
	}

	gen := GenerateOJSCode(job, "go")
	if gen.OJSType != "email.worker" {
		t.Errorf("expected email.worker, got %s", gen.OJSType)
	}
	if gen.Language != "go" {
		t.Errorf("expected go, got %s", gen.Language)
	}
	if gen.ClientCode == "" {
		t.Error("expected non-empty client code")
	}
	if gen.WorkerCode == "" {
		t.Error("expected non-empty worker code")
	}
}

func TestGenerateOJSCodeTypeScript(t *testing.T) {
	gen := GenerateOJSCode(JobDefinition{Name: "sendEmail", Queue: "email"}, "typescript")
	if gen.OJSType != "send.email" {
		t.Errorf("expected send.email, got %s", gen.OJSType)
	}
	if gen.ClientCode == "" || gen.WorkerCode == "" {
		t.Error("expected non-empty generated code")
	}
}

func TestGenerateOJSCodePython(t *testing.T) {
	gen := GenerateOJSCode(JobDefinition{Name: "process_payment", Queue: "billing"}, "python")
	if gen.ClientCode == "" || gen.WorkerCode == "" {
		t.Error("expected non-empty generated code")
	}
}

func TestGenerateMigrationPlan(t *testing.T) {
	analysis := &CodeAnalysisResult{
		Framework: FrameworkSidekiq,
		Jobs: []JobDefinition{
			{Name: "EmailWorker", Queue: "mailer"},
			{Name: "ReportGenerator", Queue: "reports"},
		},
		Queues:     []string{"mailer", "reports"},
		TotalFiles: 2,
	}

	plan := GenerateMigrationPlan(analysis, "go")

	if len(plan.Generated) != 2 {
		t.Errorf("expected 2 generated, got %d", len(plan.Generated))
	}
	if plan.Summary == "" {
		t.Error("expected non-empty summary")
	}

	data, err := plan.JSON()
	if err != nil {
		t.Fatalf("JSON: %v", err)
	}
	if len(data) == 0 {
		t.Error("expected non-empty JSON output")
	}
}

func TestCamelToSnake(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"EmailWorker", "email_worker"},
		{"sendEmail", "send_email"},
		{"process_payment", "process_payment"},
		{"HTTPServer", "h_t_t_p_server"},
		{"simple", "simple"},
	}

	for _, tt := range tests {
		got := camelToSnake(tt.input)
		if got != tt.want {
			t.Errorf("camelToSnake(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}
