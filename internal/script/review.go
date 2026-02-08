package script

import (
	"context"
	"fmt"
	"math"
	"strings"
)

// ReviewResult holds the outcome of a script review pass.
type ReviewResult struct {
	Approved bool
	Issues   []ReviewIssue
	Revised  *Script // nil if Approved
}

// ReviewIssue describes a quality problem found in the script.
type ReviewIssue struct {
	Category string // "segment_count", "format", "style", "balance", "filler"
	Message  string
	Severity string // "error" or "warning"
}

// Reviewer validates and optionally revises generated scripts.
type Reviewer struct {
	model string
}

// NewReviewer creates a reviewer that uses the same model as script generation.
func NewReviewer(model string) (*Reviewer, error) {
	return &Reviewer{model: model}, nil
}

// Review runs Phase A (heuristic checks) and optionally Phase B (LLM review).
func (r *Reviewer) Review(ctx context.Context, s *Script, content string, opts GenerateOptions) (*ReviewResult, error) {
	// Phase A: fast heuristic checks
	var issues []ReviewIssue
	issues = append(issues, checkSegmentCount(s, opts.Duration)...)
	issues = append(issues, checkSpeakerBalance(s, opts.Voices)...)
	issues = append(issues, checkFillerPhrases(s)...)

	// Determine if there are errors (not just warnings)
	hasErrors := false
	for _, issue := range issues {
		if issue.Severity == "error" {
			hasErrors = true
			break
		}
	}

	// If Phase A passes clean, skip LLM call
	if !hasErrors {
		return &ReviewResult{
			Approved: true,
			Issues:   issues, // may contain warnings
		}, nil
	}

	// Phase B: LLM review — send script + issues back to the same model
	gen, err := NewGenerator(r.model)
	if err != nil {
		// If we can't create a generator, return issues without revision
		return &ReviewResult{
			Approved: false,
			Issues:   issues,
		}, nil
	}

	reviewPrompt := buildReviewPrompt(s, content, opts, issues)
	revised, err := gen.Generate(ctx, reviewPrompt, opts)
	if err != nil {
		// LLM review failed — return heuristic issues without revision
		return &ReviewResult{
			Approved: false,
			Issues:   issues,
		}, nil
	}

	return &ReviewResult{
		Approved: false,
		Issues:   issues,
		Revised:  revised,
	}, nil
}

func checkSegmentCount(s *Script, duration string) []ReviewIssue {
	target := TargetSegments(duration)
	actual := len(s.Segments)
	tolerance := float64(target) * 0.15

	if math.Abs(float64(actual-target)) > tolerance {
		return []ReviewIssue{{
			Category: "segment_count",
			Message:  fmt.Sprintf("Script has %d segments, target is %d (±15%% tolerance: %d-%d)", actual, target, int(float64(target)-tolerance), int(float64(target)+tolerance)),
			Severity: "error",
		}}
	}
	return nil
}

func checkSpeakerBalance(s *Script, voices int) []ReviewIssue {
	if voices <= 0 {
		voices = 2
	}

	counts := map[string]int{}
	for _, seg := range s.Segments {
		counts[seg.Speaker]++
	}

	total := len(s.Segments)
	if total == 0 {
		return nil
	}

	minPct := 0.30
	if voices >= 3 {
		minPct = 0.20
	}

	var issues []ReviewIssue
	for speaker, count := range counts {
		pct := float64(count) / float64(total)
		if pct < minPct {
			issues = append(issues, ReviewIssue{
				Category: "balance",
				Message:  fmt.Sprintf("%s has only %.0f%% of segments (%d/%d), minimum is %.0f%%", speaker, pct*100, count, total, minPct*100),
				Severity: "error",
			})
		}
	}
	return issues
}

// bannedPhrases is the list of filler phrases to scan for.
var bannedPhrases = []string{
	"that's a great point",
	"absolutely",
	"exactly",
	"that's fascinating",
	"i love that",
	"so true",
	"100 percent",
	"you nailed it",
	"that's so interesting",
	"right, right",
	"great question",
	"that's a really good question",
	"i couldn't agree more",
	"you're so right",
	"that's brilliant",
	"oh wow",
	"amazing point",
	"that's spot on",
	"couldn't have said it better",
	"you hit the nail on the head",
	"that's exactly right",
}

func checkFillerPhrases(s *Script) []ReviewIssue {
	var issues []ReviewIssue
	fillerCount := 0

	for _, seg := range s.Segments {
		lower := strings.ToLower(seg.Text)
		for _, phrase := range bannedPhrases {
			if strings.Contains(lower, phrase) {
				fillerCount++
				break // count once per segment at most
			}
		}
	}

	if fillerCount > 0 {
		severity := "warning"
		if fillerCount > 5 {
			severity = "error"
		}
		issues = append(issues, ReviewIssue{
			Category: "filler",
			Message:  fmt.Sprintf("Found %d segments with banned filler phrases", fillerCount),
			Severity: severity,
		})
	}
	return issues
}

func buildReviewPrompt(s *Script, content string, opts GenerateOptions, issues []ReviewIssue) string {
	format := opts.Format
	if format == "" {
		format = "conversation"
	}

	var issueList strings.Builder
	for _, issue := range issues {
		issueList.WriteString(fmt.Sprintf("- [%s] %s: %s\n", issue.Severity, issue.Category, issue.Message))
	}

	segmentGuidance := durationToSegments(opts.Duration)

	return fmt.Sprintf(`You are reviewing and revising a podcast script. The original script has quality issues that need fixing.

ISSUES FOUND:
%s

REQUIREMENTS:
- Format: %s (%s)
- Target segments: %s
- Tone: %s
- Each speaker must have at least %s of segments
- Never use banned filler phrases like "That's a great point", "Absolutely", "Exactly", etc.

INSTRUCTIONS:
1. Fix ALL issues listed above
2. Maintain the same topic, content, and general flow
3. Keep the same speaker names and general structure
4. If segment count is wrong, add or remove segments to hit the target
5. If speaker balance is off, redistribute segments more evenly
6. Replace any filler phrases with specific, content-relevant reactions

SOURCE MATERIAL (for reference):
%s`,
		issueList.String(),
		FormatLabel(format), format,
		segmentGuidance,
		toneDescription(opts.Tone),
		speakerMinimum(opts.Voices),
		content,
	)
}

func speakerMinimum(voices int) string {
	if voices >= 3 {
		return "20%"
	}
	return "30%"
}
