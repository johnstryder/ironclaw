package planner

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"strings"
	"testing"
)

// =============================================================================
// Mock LLM Provider
// =============================================================================

// sequentialMock returns pre-configured responses in order for each call.
type sequentialMock struct {
	responses []string
	errors    []error
	callIndex int
	prompts   []string
}

func (m *sequentialMock) Generate(ctx context.Context, prompt string) (string, error) {
	m.prompts = append(m.prompts, prompt)
	idx := m.callIndex
	m.callIndex++
	if idx >= len(m.responses) {
		return "", fmt.Errorf("mock: no more responses (call %d)", idx)
	}
	var err error
	if idx < len(m.errors) && m.errors[idx] != nil {
		err = m.errors[idx]
	}
	return m.responses[idx], err
}

// =============================================================================
// ParsePlan Tests
// =============================================================================

func TestParsePlan_WhenValidJSON_ShouldReturnPlanWithSteps(t *testing.T) {
	raw := `{"steps": [{"id": 1, "description": "Research topic"}, {"id": 2, "description": "Summarize findings"}]}`
	plan, err := ParsePlan(raw, "Research and summarize")
	if err != nil {
		t.Fatalf("ParsePlan: unexpected error: %v", err)
	}
	if plan == nil {
		t.Fatal("ParsePlan: expected non-nil plan")
	}
	if plan.Goal != "Research and summarize" {
		t.Errorf("expected goal %q, got %q", "Research and summarize", plan.Goal)
	}
	if len(plan.Steps) != 2 {
		t.Fatalf("expected 2 steps, got %d", len(plan.Steps))
	}
	if plan.Steps[0].Description != "Research topic" {
		t.Errorf("step 0: expected %q, got %q", "Research topic", plan.Steps[0].Description)
	}
	if plan.Steps[1].Description != "Summarize findings" {
		t.Errorf("step 1: expected %q, got %q", "Summarize findings", plan.Steps[1].Description)
	}
}

func TestParsePlan_WhenValidJSON_ShouldSetAllStepsToPending(t *testing.T) {
	raw := `{"steps": [{"id": 1, "description": "Step A"}, {"id": 2, "description": "Step B"}]}`
	plan, err := ParsePlan(raw, "goal")
	if err != nil {
		t.Fatalf("ParsePlan: %v", err)
	}
	for i, step := range plan.Steps {
		if step.Status != StepPending {
			t.Errorf("step %d: expected status %q, got %q", i, StepPending, step.Status)
		}
	}
}

func TestParsePlan_WhenMarkdownWrappedJSON_ShouldExtractAndParse(t *testing.T) {
	raw := "Here's my plan:\n```json\n{\"steps\": [{\"id\": 1, \"description\": \"Do thing\"}]}\n```\n"
	plan, err := ParsePlan(raw, "goal")
	if err != nil {
		t.Fatalf("ParsePlan: %v", err)
	}
	if plan == nil {
		t.Fatal("expected non-nil plan")
	}
	if len(plan.Steps) != 1 {
		t.Fatalf("expected 1 step, got %d", len(plan.Steps))
	}
	if plan.Steps[0].Description != "Do thing" {
		t.Errorf("expected %q, got %q", "Do thing", plan.Steps[0].Description)
	}
}

func TestParsePlan_WhenCodeBlockWithoutLangTag_ShouldExtractAndParse(t *testing.T) {
	raw := "```\n{\"steps\": [{\"id\": 1, \"description\": \"Step one\"}]}\n```"
	plan, err := ParsePlan(raw, "goal")
	if err != nil {
		t.Fatalf("ParsePlan: %v", err)
	}
	if len(plan.Steps) != 1 {
		t.Fatalf("expected 1 step, got %d", len(plan.Steps))
	}
}

func TestParsePlan_WhenInvalidJSON_ShouldReturnError(t *testing.T) {
	raw := "this is not json at all"
	_, err := ParsePlan(raw, "goal")
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}

func TestParsePlan_WhenEmptySteps_ShouldReturnError(t *testing.T) {
	raw := `{"steps": []}`
	_, err := ParsePlan(raw, "goal")
	if err == nil {
		t.Error("expected error for empty steps")
	}
}

func TestParsePlan_WhenEmptyInput_ShouldReturnError(t *testing.T) {
	_, err := ParsePlan("", "goal")
	if err == nil {
		t.Error("expected error for empty input")
	}
}

func TestParsePlan_WhenStepsMissingDescription_ShouldReturnError(t *testing.T) {
	raw := `{"steps": [{"id": 1, "description": ""}, {"id": 2, "description": "valid"}]}`
	_, err := ParsePlan(raw, "goal")
	if err == nil {
		t.Error("expected error when step has empty description")
	}
}

func TestParsePlan_ShouldAssignSequentialIDs(t *testing.T) {
	raw := `{"steps": [{"id": 99, "description": "A"}, {"id": 5, "description": "B"}]}`
	plan, err := ParsePlan(raw, "goal")
	if err != nil {
		t.Fatalf("ParsePlan: %v", err)
	}
	// IDs should be normalized to 1-based sequential
	for i, step := range plan.Steps {
		if step.ID != i+1 {
			t.Errorf("step %d: expected ID %d, got %d", i, i+1, step.ID)
		}
	}
}

func TestParsePlan_WhenJSONHasExtraTextAround_ShouldExtractJSON(t *testing.T) {
	raw := "Sure! Here is my plan:\n{\"steps\": [{\"id\": 1, \"description\": \"First step\"}]}\nLet me know if you need changes."
	plan, err := ParsePlan(raw, "goal")
	if err != nil {
		t.Fatalf("ParsePlan: %v", err)
	}
	if len(plan.Steps) != 1 {
		t.Fatalf("expected 1 step, got %d", len(plan.Steps))
	}
}

// =============================================================================
// BuildPlanPrompt Tests
// =============================================================================

func TestBuildPlanPrompt_ShouldContainGoal(t *testing.T) {
	prompt := BuildPlanPrompt("Research quantum computing")
	if !strings.Contains(prompt, "Research quantum computing") {
		t.Errorf("expected prompt to contain the goal, got %q", prompt)
	}
}

func TestBuildPlanPrompt_ShouldRequestJSONFormat(t *testing.T) {
	prompt := BuildPlanPrompt("any goal")
	if !strings.Contains(prompt, "JSON") && !strings.Contains(prompt, "json") {
		t.Errorf("expected prompt to mention JSON format, got %q", prompt)
	}
}

func TestBuildPlanPrompt_ShouldRequestStepsArray(t *testing.T) {
	prompt := BuildPlanPrompt("any goal")
	if !strings.Contains(prompt, "steps") {
		t.Errorf("expected prompt to mention 'steps', got %q", prompt)
	}
}

func TestBuildPlanPrompt_ShouldRequestIDAndDescription(t *testing.T) {
	prompt := BuildPlanPrompt("any goal")
	if !strings.Contains(prompt, "id") {
		t.Errorf("expected prompt to mention 'id', got %q", prompt)
	}
	if !strings.Contains(prompt, "description") {
		t.Errorf("expected prompt to mention 'description', got %q", prompt)
	}
}

// =============================================================================
// BuildStepPrompt Tests
// =============================================================================

func TestBuildStepPrompt_ShouldContainGoal(t *testing.T) {
	plan := &Plan{
		Goal: "Build a rocket",
		Steps: []PlanStep{
			{ID: 1, Description: "Design engine", Status: StepInProgress},
		},
	}
	prompt := BuildStepPrompt(plan, 0)
	if !strings.Contains(prompt, "Build a rocket") {
		t.Errorf("expected prompt to contain goal, got %q", prompt)
	}
}

func TestBuildStepPrompt_ShouldContainCurrentStepDescription(t *testing.T) {
	plan := &Plan{
		Goal: "goal",
		Steps: []PlanStep{
			{ID: 1, Description: "First step", Status: StepCompleted, Result: "done"},
			{ID: 2, Description: "Second step", Status: StepInProgress},
		},
	}
	prompt := BuildStepPrompt(plan, 1)
	if !strings.Contains(prompt, "Second step") {
		t.Errorf("expected prompt to contain current step description, got %q", prompt)
	}
}

func TestBuildStepPrompt_ShouldShowCompletedStepResults(t *testing.T) {
	plan := &Plan{
		Goal: "goal",
		Steps: []PlanStep{
			{ID: 1, Description: "Research", Status: StepCompleted, Result: "Found 5 papers"},
			{ID: 2, Description: "Summarize", Status: StepInProgress},
		},
	}
	prompt := BuildStepPrompt(plan, 1)
	if !strings.Contains(prompt, "Found 5 papers") {
		t.Errorf("expected prompt to contain previous step result, got %q", prompt)
	}
}

func TestBuildStepPrompt_ShouldShowPlanStatus(t *testing.T) {
	plan := &Plan{
		Goal: "goal",
		Steps: []PlanStep{
			{ID: 1, Description: "Done step", Status: StepCompleted, Result: "ok"},
			{ID: 2, Description: "Current", Status: StepInProgress},
			{ID: 3, Description: "Future", Status: StepPending},
		},
	}
	prompt := BuildStepPrompt(plan, 1)
	if !strings.Contains(prompt, "completed") {
		t.Errorf("expected prompt to show completed status, got %q", prompt)
	}
	if !strings.Contains(prompt, "in_progress") {
		t.Errorf("expected prompt to show in_progress status, got %q", prompt)
	}
	if !strings.Contains(prompt, "pending") {
		t.Errorf("expected prompt to show pending status, got %q", prompt)
	}
}

// =============================================================================
// BuildSummaryPrompt Tests
// =============================================================================

func TestBuildSummaryPrompt_ShouldContainGoal(t *testing.T) {
	plan := &Plan{
		Goal: "Conquer the world",
		Steps: []PlanStep{
			{ID: 1, Description: "Step 1", Status: StepCompleted, Result: "Done"},
		},
	}
	prompt := BuildSummaryPrompt(plan)
	if !strings.Contains(prompt, "Conquer the world") {
		t.Errorf("expected prompt to contain goal, got %q", prompt)
	}
}

func TestBuildSummaryPrompt_ShouldContainAllStepResults(t *testing.T) {
	plan := &Plan{
		Goal: "goal",
		Steps: []PlanStep{
			{ID: 1, Description: "Step A", Status: StepCompleted, Result: "Result A"},
			{ID: 2, Description: "Step B", Status: StepCompleted, Result: "Result B"},
			{ID: 3, Description: "Step C", Status: StepFailed, Result: "Error C"},
		},
	}
	prompt := BuildSummaryPrompt(plan)
	if !strings.Contains(prompt, "Result A") {
		t.Errorf("expected prompt to contain Result A, got %q", prompt)
	}
	if !strings.Contains(prompt, "Result B") {
		t.Errorf("expected prompt to contain Result B, got %q", prompt)
	}
	if !strings.Contains(prompt, "Error C") {
		t.Errorf("expected prompt to contain Error C, got %q", prompt)
	}
}

func TestBuildSummaryPrompt_ShouldMentionSummary(t *testing.T) {
	plan := &Plan{
		Goal:  "goal",
		Steps: []PlanStep{{ID: 1, Description: "S", Status: StepCompleted, Result: "R"}},
	}
	prompt := BuildSummaryPrompt(plan)
	lower := strings.ToLower(prompt)
	if !strings.Contains(lower, "summar") {
		t.Errorf("expected prompt to mention summary, got %q", prompt)
	}
}

// =============================================================================
// NewPlanner Constructor Tests
// =============================================================================

func TestNewPlanner_ShouldReturnPlannerWithDefaults(t *testing.T) {
	mock := &sequentialMock{responses: []string{"ok"}}
	p := NewPlanner(mock)
	if p == nil {
		t.Fatal("expected non-nil planner")
	}
	if p.maxIterations != DefaultMaxIterations {
		t.Errorf("expected default max iterations %d, got %d", DefaultMaxIterations, p.maxIterations)
	}
}

func TestNewPlanner_WhenNilLLM_ShouldPanic(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Error("NewPlanner(nil) should panic")
		}
	}()
	NewPlanner(nil)
}

func TestNewPlanner_WithMaxIterations_ShouldOverrideDefault(t *testing.T) {
	mock := &sequentialMock{responses: []string{"ok"}}
	p := NewPlanner(mock, WithMaxIterations(5))
	if p.maxIterations != 5 {
		t.Errorf("expected max iterations 5, got %d", p.maxIterations)
	}
}

func TestNewPlanner_WithMaxIterationsZero_ShouldKeepDefault(t *testing.T) {
	mock := &sequentialMock{responses: []string{"ok"}}
	p := NewPlanner(mock, WithMaxIterations(0))
	if p.maxIterations != DefaultMaxIterations {
		t.Errorf("expected default max iterations, got %d", p.maxIterations)
	}
}

func TestNewPlanner_WithNilLogger_ShouldNotPanic(t *testing.T) {
	mock := &sequentialMock{responses: []string{"ok"}}
	p := NewPlanner(mock, WithLogger(nil))
	if p == nil {
		t.Fatal("expected non-nil planner")
	}
}

// =============================================================================
// Execute Loop Tests
// =============================================================================

func TestExecute_HappyPath_ShouldDecomposeAndExecuteAllSteps(t *testing.T) {
	// Call 1: LLM returns a plan
	// Call 2: LLM executes step 1
	// Call 3: LLM executes step 2
	// Call 4: LLM generates summary
	mock := &sequentialMock{
		responses: []string{
			`{"steps": [{"id": 1, "description": "Research the topic"}, {"id": 2, "description": "Write summary"}]}`,
			"Found 3 key papers on the subject.",
			"Summary: The topic covers X, Y, and Z.",
			"Final summary: Research completed with 3 papers reviewed covering X, Y, Z.",
		},
	}

	p := NewPlanner(mock)
	result, err := p.Execute(context.Background(), "Research and summarize quantum computing")
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil result")
	}

	// Should have executed both steps
	if result.StepsExecuted != 2 {
		t.Errorf("expected 2 steps executed, got %d", result.StepsExecuted)
	}

	// Plan should have both steps completed
	if len(result.Plan.Steps) != 2 {
		t.Fatalf("expected 2 steps in plan, got %d", len(result.Plan.Steps))
	}
	for i, step := range result.Plan.Steps {
		if step.Status != StepCompleted {
			t.Errorf("step %d: expected status %q, got %q", i, StepCompleted, step.Status)
		}
	}

	// Step results should be populated
	if result.Plan.Steps[0].Result != "Found 3 key papers on the subject." {
		t.Errorf("step 0 result: got %q", result.Plan.Steps[0].Result)
	}
	if result.Plan.Steps[1].Result != "Summary: The topic covers X, Y, and Z." {
		t.Errorf("step 1 result: got %q", result.Plan.Steps[1].Result)
	}

	// Final summary should be populated
	if result.FinalSummary == "" {
		t.Error("expected non-empty final summary")
	}

	// Should have made 4 LLM calls: plan + 2 steps + summary
	if mock.callIndex != 4 {
		t.Errorf("expected 4 LLM calls, got %d", mock.callIndex)
	}
}

func TestExecute_ShouldSetGoalOnPlan(t *testing.T) {
	mock := &sequentialMock{
		responses: []string{
			`{"steps": [{"id": 1, "description": "Step one"}]}`,
			"Step one result",
			"Summary done",
		},
	}
	p := NewPlanner(mock)
	result, err := p.Execute(context.Background(), "My specific goal")
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if result.Plan.Goal != "My specific goal" {
		t.Errorf("expected goal %q, got %q", "My specific goal", result.Plan.Goal)
	}
}

func TestExecute_ShouldFeedPreviousStepResultsIntoContext(t *testing.T) {
	mock := &sequentialMock{
		responses: []string{
			`{"steps": [{"id": 1, "description": "Step A"}, {"id": 2, "description": "Step B"}]}`,
			"Result from step A",
			"Result from step B",
			"Final summary",
		},
	}

	p := NewPlanner(mock)
	_, err := p.Execute(context.Background(), "goal")
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}

	// The prompt for step 2 (call index 2) should contain step 1's result
	if len(mock.prompts) < 3 {
		t.Fatalf("expected at least 3 prompts, got %d", len(mock.prompts))
	}
	stepBPrompt := mock.prompts[2]
	if !strings.Contains(stepBPrompt, "Result from step A") {
		t.Errorf("step B prompt should contain step A's result, got %q", stepBPrompt)
	}
}

func TestExecute_WhenPlanGenerationFails_ShouldReturnError(t *testing.T) {
	mock := &sequentialMock{
		responses: []string{""},
		errors:    []error{errors.New("LLM unavailable")},
	}
	p := NewPlanner(mock)
	_, err := p.Execute(context.Background(), "goal")
	if err == nil {
		t.Error("expected error when plan generation fails")
	}
	if !strings.Contains(err.Error(), "LLM unavailable") {
		t.Errorf("expected error to contain 'LLM unavailable', got %q", err.Error())
	}
}

func TestExecute_WhenPlanParsingFails_ShouldReturnError(t *testing.T) {
	mock := &sequentialMock{
		responses: []string{"This is not valid JSON plan at all with no braces"},
	}
	p := NewPlanner(mock)
	_, err := p.Execute(context.Background(), "goal")
	if err == nil {
		t.Error("expected error when plan parsing fails")
	}
}

func TestExecute_WhenStepExecutionFails_ShouldMarkStepAsFailedAndContinue(t *testing.T) {
	mock := &sequentialMock{
		responses: []string{
			`{"steps": [{"id": 1, "description": "Failing step"}, {"id": 2, "description": "Good step"}]}`,
			"",    // step 1 fails
			"ok",  // step 2 succeeds
			"Final summary with partial success",
		},
		errors: []error{
			nil,                          // plan generation succeeds
			errors.New("step 1 failed"), // step 1 fails
			nil,                          // step 2 succeeds
			nil,                          // summary succeeds
		},
	}

	p := NewPlanner(mock)
	result, err := p.Execute(context.Background(), "goal")
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}

	// Step 1 should be marked as failed
	if result.Plan.Steps[0].Status != StepFailed {
		t.Errorf("step 0: expected %q, got %q", StepFailed, result.Plan.Steps[0].Status)
	}
	// Step 2 should be completed
	if result.Plan.Steps[1].Status != StepCompleted {
		t.Errorf("step 1: expected %q, got %q", StepCompleted, result.Plan.Steps[1].Status)
	}
	// Both steps were attempted
	if result.StepsExecuted != 2 {
		t.Errorf("expected 2 steps executed, got %d", result.StepsExecuted)
	}
}

func TestExecute_WhenContextCanceledBeforeStep_ShouldReturnPartialResult(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())

	wrapper := &contextCancellingMock{
		responses: []string{
			`{"steps": [{"id": 1, "description": "Step 1"}, {"id": 2, "description": "Step 2"}, {"id": 3, "description": "Step 3"}]}`,
			"Step 1 done",
			"Step 2 done",
			"Step 3 done",
			"Summary",
		},
		cancelAfter: 1, // cancel after call index 1 (step 1 execution)
		cancelFunc:  cancel,
	}

	p := NewPlanner(wrapper)
	result, err := p.Execute(ctx, "goal")

	if err != nil {
		t.Fatalf("Execute should not return error on cancel, got: %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil partial result")
	}
	// Step 1 should be completed
	if result.Plan.Steps[0].Status != StepCompleted {
		t.Errorf("step 0: expected %q, got %q", StepCompleted, result.Plan.Steps[0].Status)
	}
	if result.StepsExecuted < 1 {
		t.Errorf("expected at least 1 step executed, got %d", result.StepsExecuted)
	}
}

// contextCancellingMock cancels the context after a specified number of Generate calls.
type contextCancellingMock struct {
	responses   []string
	cancelAfter int
	cancelFunc  context.CancelFunc
	callIndex   int
	prompts     []string
}

func (m *contextCancellingMock) Generate(ctx context.Context, prompt string) (string, error) {
	m.prompts = append(m.prompts, prompt)
	idx := m.callIndex
	m.callIndex++

	if idx > m.cancelAfter {
		m.cancelFunc()
	}

	if idx >= len(m.responses) {
		return "", fmt.Errorf("mock: no more responses")
	}
	return m.responses[idx], nil
}

func TestExecute_WhenMaxIterationsReached_ShouldStopExecution(t *testing.T) {
	mock := &sequentialMock{
		responses: []string{
			`{"steps": [{"id": 1, "description": "S1"}, {"id": 2, "description": "S2"}, {"id": 3, "description": "S3"}, {"id": 4, "description": "S4"}, {"id": 5, "description": "S5"}]}`,
			"R1",
			"R2",
			// maxIterations=2: only 2 steps should be executed
			"Partial summary",
		},
	}

	p := NewPlanner(mock, WithMaxIterations(2))
	result, err := p.Execute(context.Background(), "goal")
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}

	if result.StepsExecuted != 2 {
		t.Errorf("expected 2 steps executed (max iterations), got %d", result.StepsExecuted)
	}
	// Steps 3-5 should remain pending
	if result.Plan.Steps[2].Status != StepPending {
		t.Errorf("step 2: expected %q, got %q", StepPending, result.Plan.Steps[2].Status)
	}
}

func TestExecute_SingleStep_ShouldWorkCorrectly(t *testing.T) {
	mock := &sequentialMock{
		responses: []string{
			`{"steps": [{"id": 1, "description": "Only step"}]}`,
			"Single result",
			"Summary of single step",
		},
	}

	p := NewPlanner(mock)
	result, err := p.Execute(context.Background(), "simple goal")
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if result.StepsExecuted != 1 {
		t.Errorf("expected 1 step executed, got %d", result.StepsExecuted)
	}
	if result.FinalSummary != "Summary of single step" {
		t.Errorf("expected summary %q, got %q", "Summary of single step", result.FinalSummary)
	}
}

// =============================================================================
// Edge Cases & Additional Coverage Tests
// =============================================================================

func TestExecute_WhenSummaryGenerationFails_ShouldReturnFallbackSummary(t *testing.T) {
	mock := &sequentialMock{
		responses: []string{
			`{"steps": [{"id": 1, "description": "Step 1"}]}`,
			"Step 1 result",
			"", // summary fails
		},
		errors: []error{nil, nil, errors.New("summary LLM down")},
	}

	p := NewPlanner(mock)
	result, err := p.Execute(context.Background(), "goal")
	if err != nil {
		t.Fatalf("Execute should not fail: %v", err)
	}
	if !strings.Contains(result.FinalSummary, "Summary generation failed") {
		t.Errorf("expected fallback summary message, got %q", result.FinalSummary)
	}
	// Steps should still be completed
	if result.Plan.Steps[0].Status != StepCompleted {
		t.Errorf("step should be completed despite summary failure")
	}
}

func TestExecute_WhenAllStepsFail_ShouldStillReturnResult(t *testing.T) {
	mock := &sequentialMock{
		responses: []string{
			`{"steps": [{"id": 1, "description": "S1"}, {"id": 2, "description": "S2"}]}`,
			"",
			"",
			"Summary of failures",
		},
		errors: []error{nil, errors.New("fail 1"), errors.New("fail 2"), nil},
	}

	p := NewPlanner(mock)
	result, err := p.Execute(context.Background(), "doomed goal")
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	for i, step := range result.Plan.Steps {
		if step.Status != StepFailed {
			t.Errorf("step %d: expected %q, got %q", i, StepFailed, step.Status)
		}
	}
	if result.StepsExecuted != 2 {
		t.Errorf("expected 2 steps attempted, got %d", result.StepsExecuted)
	}
}

func TestExecute_WhenStepFails_ShouldRecordErrorInResult(t *testing.T) {
	mock := &sequentialMock{
		responses: []string{
			`{"steps": [{"id": 1, "description": "Boom"}]}`,
			"",
			"Summary",
		},
		errors: []error{nil, errors.New("kaboom"), nil},
	}

	p := NewPlanner(mock)
	result, err := p.Execute(context.Background(), "goal")
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if !strings.Contains(result.Plan.Steps[0].Result, "kaboom") {
		t.Errorf("expected error recorded in step result, got %q", result.Plan.Steps[0].Result)
	}
}

func TestExecute_WhenContextAlreadyCanceled_ShouldReturnErrorFromPlanGeneration(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	// Use a context-aware mock
	mock := &contextAwareMock{}
	p := NewPlanner(mock)
	_, err := p.Execute(ctx, "goal")
	if err == nil {
		t.Error("expected error when context is already canceled")
	}
}

// contextAwareMock respects context cancellation.
type contextAwareMock struct{}

func (m *contextAwareMock) Generate(ctx context.Context, prompt string) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	return "ok", nil
}

func TestExtractJSON_WhenNoJSON_ShouldReturnEmpty(t *testing.T) {
	result := extractJSON("no json here at all")
	if result != "" {
		t.Errorf("expected empty string, got %q", result)
	}
}

func TestExtractJSON_WhenCodeBlockHasNoClosing_ShouldReturnEmpty(t *testing.T) {
	result := extractJSON("```json\n{\"steps\": []}")
	if result != "" {
		t.Errorf("expected empty string for unclosed code block, got %q", result)
	}
}

func TestExtractJSON_WhenCodeBlockHasNoNewline_ShouldReturnEmpty(t *testing.T) {
	result := extractJSON("```")
	if result != "" {
		t.Errorf("expected empty string, got %q", result)
	}
}

func TestParsePlan_WhenWhitespaceOnly_ShouldReturnError(t *testing.T) {
	_, err := ParsePlan("   \n\t  ", "goal")
	if err == nil {
		t.Error("expected error for whitespace-only input")
	}
}

func TestExecute_WithLogger_ShouldNotPanic(t *testing.T) {
	mock := &sequentialMock{
		responses: []string{
			`{"steps": [{"id": 1, "description": "Step"}]}`,
			"Result",
			"Summary",
		},
	}

	// Use a custom logger - just verify it doesn't panic
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	p := NewPlanner(mock, WithLogger(logger))
	result, err := p.Execute(context.Background(), "test goal")
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if result.StepsExecuted != 1 {
		t.Errorf("expected 1 step, got %d", result.StepsExecuted)
	}
}

func TestExecute_WhenMaxIterationsIsOne_ShouldExecuteOnlyOneStep(t *testing.T) {
	mock := &sequentialMock{
		responses: []string{
			`{"steps": [{"id": 1, "description": "S1"}, {"id": 2, "description": "S2"}, {"id": 3, "description": "S3"}]}`,
			"R1",
			"Summary after 1 step",
		},
	}

	p := NewPlanner(mock, WithMaxIterations(1))
	result, err := p.Execute(context.Background(), "goal")
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if result.StepsExecuted != 1 {
		t.Errorf("expected 1 step, got %d", result.StepsExecuted)
	}
	if result.Plan.Steps[0].Status != StepCompleted {
		t.Errorf("step 0 should be completed")
	}
	if result.Plan.Steps[1].Status != StepPending {
		t.Errorf("step 1 should be pending")
	}
	if result.Plan.Steps[2].Status != StepPending {
		t.Errorf("step 2 should be pending")
	}
}

func TestParsePlan_WhenMalformedJSONInCodeBlock_ShouldReturnError(t *testing.T) {
	raw := "```json\n{not valid json}\n```"
	_, err := ParsePlan(raw, "goal")
	if err == nil {
		t.Error("expected error for malformed JSON in code block")
	}
}

func TestBuildStepPrompt_ShouldShowStepNumber(t *testing.T) {
	plan := &Plan{
		Goal: "goal",
		Steps: []PlanStep{
			{ID: 1, Description: "Step one", Status: StepInProgress},
		},
	}
	prompt := BuildStepPrompt(plan, 0)
	if !strings.Contains(prompt, "step 1") {
		t.Errorf("expected prompt to contain step number, got %q", prompt)
	}
}

func TestBuildSummaryPrompt_ShouldShowStepStatuses(t *testing.T) {
	plan := &Plan{
		Goal: "goal",
		Steps: []PlanStep{
			{ID: 1, Description: "S1", Status: StepCompleted, Result: "R1"},
			{ID: 2, Description: "S2", Status: StepFailed, Result: "Error"},
		},
	}
	prompt := BuildSummaryPrompt(plan)
	if !strings.Contains(prompt, string(StepCompleted)) {
		t.Errorf("expected completed status in prompt")
	}
	if !strings.Contains(prompt, string(StepFailed)) {
		t.Errorf("expected failed status in prompt")
	}
}
