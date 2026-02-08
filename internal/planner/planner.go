package planner

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"

	"ironclaw/internal/domain"
)

// StepStatus represents the execution state of a plan step.
type StepStatus string

const (
	StepPending    StepStatus = "pending"
	StepInProgress StepStatus = "in_progress"
	StepCompleted  StepStatus = "completed"
	StepFailed     StepStatus = "failed"
)

// PlanStep is a single actionable step in a plan.
type PlanStep struct {
	ID          int        `json:"id"`
	Description string     `json:"description"`
	Status      StepStatus `json:"status"`
	Result      string     `json:"result,omitempty"`
}

// Plan is a goal decomposed into ordered steps.
type Plan struct {
	Goal  string     `json:"goal"`
	Steps []PlanStep `json:"steps"`
}

// PlanResult is the outcome of executing a plan.
type PlanResult struct {
	Plan          Plan
	FinalSummary  string
	StepsExecuted int
}

// Option configures a Planner.
type Option func(*Planner)

// WithMaxIterations sets the maximum number of step execution iterations.
func WithMaxIterations(n int) Option {
	return func(p *Planner) {
		if n > 0 {
			p.maxIterations = n
		}
	}
}

// WithLogger sets a structured logger for the Planner.
func WithLogger(l *slog.Logger) Option {
	return func(p *Planner) {
		if l != nil {
			p.logger = l
		}
	}
}

// DefaultMaxIterations is the default cap on plan execution steps.
const DefaultMaxIterations = 10

// Planner decomposes complex goals into steps and executes them sequentially,
// feeding each step's result back into context before executing the next.
type Planner struct {
	llm           domain.LLMProvider
	maxIterations int
	logger        *slog.Logger
}

// NewPlanner creates a Planner backed by the given LLM provider.
// Panics if llm is nil.
func NewPlanner(llm domain.LLMProvider, opts ...Option) *Planner {
	if llm == nil {
		panic("planner: llm provider must not be nil")
	}
	p := &Planner{
		llm:           llm,
		maxIterations: DefaultMaxIterations,
	}
	for _, opt := range opts {
		opt(p)
	}
	return p
}

// log returns the Planner's logger, falling back to the default slog logger.
func (p *Planner) log() *slog.Logger {
	if p.logger != nil {
		return p.logger
	}
	return slog.Default()
}

// Execute decomposes the goal into steps and executes them sequentially.
// Each step's result is fed back into the context for the next step.
// Returns a PlanResult with the final plan state, summary, and step count.
func (p *Planner) Execute(ctx context.Context, goal string) (*PlanResult, error) {
	// Phase 1: Generate the plan.
	planPrompt := BuildPlanPrompt(goal)
	p.log().Info("generating plan", "goal", goal)

	planResponse, err := p.llm.Generate(ctx, planPrompt)
	if err != nil {
		return nil, fmt.Errorf("planner: failed to generate plan: %w", err)
	}

	plan, err := ParsePlan(planResponse, goal)
	if err != nil {
		return nil, fmt.Errorf("planner: failed to parse plan: %w", err)
	}

	p.log().Info("plan generated", "steps", len(plan.Steps))

	// Phase 2: Execute steps sequentially.
	stepsExecuted := 0
	for i := range plan.Steps {
		// Check context cancellation before each step.
		if ctx.Err() != nil {
			p.log().Warn("context canceled, returning partial result", "steps_executed", stepsExecuted)
			return &PlanResult{
				Plan:          *plan,
				FinalSummary:  "Execution interrupted: " + ctx.Err().Error(),
				StepsExecuted: stepsExecuted,
			}, nil
		}

		// Check max iterations.
		if stepsExecuted >= p.maxIterations {
			p.log().Warn("max iterations reached", "limit", p.maxIterations, "steps_executed", stepsExecuted)
			break
		}

		plan.Steps[i].Status = StepInProgress
		stepPrompt := BuildStepPrompt(plan, i)

		p.log().Info("executing step", "step", i+1, "description", plan.Steps[i].Description)

		stepResult, stepErr := p.llm.Generate(ctx, stepPrompt)
		stepsExecuted++

		if stepErr != nil {
			plan.Steps[i].Status = StepFailed
			plan.Steps[i].Result = "Error: " + stepErr.Error()
			p.log().Warn("step failed", "step", i+1, "error", stepErr)
			continue
		}

		plan.Steps[i].Status = StepCompleted
		plan.Steps[i].Result = stepResult
		p.log().Info("step completed", "step", i+1)
	}

	// Phase 3: Generate final summary.
	summaryPrompt := BuildSummaryPrompt(plan)
	summary, summaryErr := p.llm.Generate(ctx, summaryPrompt)
	if summaryErr != nil {
		// Summary generation failure is non-fatal; use a fallback.
		summary = "Summary generation failed: " + summaryErr.Error()
		p.log().Warn("summary generation failed", "error", summaryErr)
	}

	return &PlanResult{
		Plan:          *plan,
		FinalSummary:  summary,
		StepsExecuted: stepsExecuted,
	}, nil
}

// ParsePlan extracts a Plan from an LLM response string.
// Handles raw JSON, markdown-wrapped code blocks, and JSON embedded in prose.
func ParsePlan(raw string, goal string) (*Plan, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, fmt.Errorf("planner: empty response")
	}

	jsonStr := extractJSON(raw)
	if jsonStr == "" {
		return nil, fmt.Errorf("planner: no valid JSON found in response")
	}

	var parsed struct {
		Steps []struct {
			ID          int    `json:"id"`
			Description string `json:"description"`
		} `json:"steps"`
	}
	if err := json.Unmarshal([]byte(jsonStr), &parsed); err != nil {
		return nil, fmt.Errorf("planner: failed to parse plan JSON: %w", err)
	}

	if len(parsed.Steps) == 0 {
		return nil, fmt.Errorf("planner: plan has no steps")
	}

	steps := make([]PlanStep, len(parsed.Steps))
	for i, s := range parsed.Steps {
		desc := strings.TrimSpace(s.Description)
		if desc == "" {
			return nil, fmt.Errorf("planner: step %d has empty description", i+1)
		}
		steps[i] = PlanStep{
			ID:          i + 1, // normalize to 1-based sequential
			Description: desc,
			Status:      StepPending,
		}
	}

	return &Plan{Goal: goal, Steps: steps}, nil
}

// extractJSON tries to find a JSON object in the response. It checks for:
// 1. Markdown code blocks (```json ... ``` or ``` ... ```)
// 2. Raw JSON object
// 3. JSON object embedded in prose text
func extractJSON(raw string) string {
	// Try markdown code block first.
	if idx := strings.Index(raw, "```"); idx >= 0 {
		start := strings.Index(raw[idx:], "\n")
		if start < 0 {
			return ""
		}
		start += idx + 1
		end := strings.Index(raw[start:], "```")
		if end < 0 {
			return ""
		}
		return strings.TrimSpace(raw[start : start+end])
	}

	// Try raw JSON: find first { and last }.
	braceStart := strings.Index(raw, "{")
	braceEnd := strings.LastIndex(raw, "}")
	if braceStart >= 0 && braceEnd > braceStart {
		return raw[braceStart : braceEnd+1]
	}

	return ""
}

// BuildPlanPrompt creates the prompt that asks the LLM to decompose a goal into steps.
func BuildPlanPrompt(goal string) string {
	return fmt.Sprintf(`You are a task planner. Break down the following goal into clear, actionable steps.

Return ONLY a JSON object with this exact format:
{"steps": [{"id": 1, "description": "Step description"}, {"id": 2, "description": "Next step"}, ...]}

Rules:
- Each step must have an "id" (integer) and "description" (string).
- Steps should be ordered logically.
- Keep steps concrete and actionable.
- Use 2-7 steps for most goals.

Goal: %s`, goal)
}

// BuildStepPrompt creates the prompt for executing a specific step with full plan context.
func BuildStepPrompt(plan *Plan, stepIndex int) string {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("You are executing a multi-step plan.\n\nGoal: %s\n\n", plan.Goal))
	sb.WriteString("Current plan status:\n")
	writeStepList(&sb, plan.Steps)

	current := plan.Steps[stepIndex]
	sb.WriteString(fmt.Sprintf("\nNow execute step %d: %s\n", current.ID, current.Description))
	sb.WriteString("Provide a detailed result for this step.")

	return sb.String()
}

// BuildSummaryPrompt creates the prompt for generating a final summary of the completed plan.
func BuildSummaryPrompt(plan *Plan) string {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("You completed a multi-step plan. Provide a final summary of the results.\n\nGoal: %s\n\n", plan.Goal))
	sb.WriteString("Steps completed:\n")
	writeStepList(&sb, plan.Steps)
	sb.WriteString("\nSummarize what was accomplished and any notable findings or outcomes.")

	return sb.String()
}

// writeStepList writes a formatted list of steps with their statuses and results.
func writeStepList(sb *strings.Builder, steps []PlanStep) {
	for _, step := range steps {
		sb.WriteString(fmt.Sprintf("  %d. [%s] %s", step.ID, step.Status, step.Description))
		if step.Result != "" {
			sb.WriteString(fmt.Sprintf("\n     Result: %s", step.Result))
		}
		sb.WriteString("\n")
	}
}
