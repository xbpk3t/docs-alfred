// Package ghcheck validates data/gh YAML topic.kind values for data-cli check gh.
package ghcheck

import (
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/goccy/go-yaml"
	"github.com/gookit/validate"
	"github.com/xbpk3t/docs-alfred/pkg/checkutil"
	"github.com/xbpk3t/docs-alfred/pkg/fileutil"
)

// Allowed topic.kind values for data/gh (gate; unset is not allowed).
const (
	KindMechanism = "mech"
	KindType      = "type"
	KindRepo      = "repo"
	KindTools     = "tools"
	KindHowto     = "howto"
	KindTemp      = "temp"

	// MaxTopicsPerSection is the max topics allowed under one section type.
	MaxTopicsPerSection = 30
)

// AllowedKindsCSV is the allowed set shown in error messages.
const AllowedKindsCSV = "mech|type|repo|tools|howto|temp"

// CheckResult holds gh validation issues.
type CheckResult struct {
	Issues []checkutil.Issue
}

// --- YAML DTOs (unmarshal only; no validate tags — avoids gookit nil-nested required pitfall) ---

type sectionDTO struct {
	Type   string     `yaml:"type"`
	Topics []topicDTO `yaml:"topics"`
}

type topicDTO struct {
	Mdscc *mdsccDTO `yaml:"mdscc"`
	Topic string    `yaml:"topic"`
	Kind  string    `yaml:"kind"`
}

type mdsccDTO struct {
	Meta   string `yaml:"meta"`
	Derive string `yaml:"derive"`
	Sol    string `yaml:"sol"`
	Cost   string `yaml:"cost"`
	Case   string `yaml:"case"`
}

// --- validate views (rules only; mdscc validated separately when non-nil) ---

type sectionRules struct {
	Type   string     `validate:"required" filter:"trim" message:"required:type is required"`
	Topics []struct{} `validate:"required|minLen:1|maxLen:30" message:"required:topics is required|minLen:topics is required|maxLen:too many topics"`
}

type topicRules struct {
	Topic string `validate:"required" filter:"trim" message:"required:topic is required"`
	Kind  string `validate:"required|in:mech,type,repo,tools,howto,temp" filter:"trim" message:"required:kind is required"`
}

type mdsccRules struct {
	Meta   string `validate:"required" filter:"trim" message:"required:mdscc.meta is required"`
	Derive string `validate:"required" filter:"trim" message:"required:mdscc.derive is required"`
	Sol    string `validate:"required" filter:"trim" message:"required:mdscc.sol is required"`
	Cost   string `validate:"required" filter:"trim" message:"required:mdscc.cost is required"`
	Case   string `validate:"required" filter:"trim" message:"required:mdscc.case is required"`
}

// RunCheck validates topic.kind on all YAML files under ghRoot (recursive).
// Missing path returns a Go error; per-file YAML/shape problems become Issues.
func RunCheck(ghRoot string) (*CheckResult, error) {
	files, err := fileutil.ListYAMLFilesRecursive(ghRoot)
	if err != nil {
		return nil, fmt.Errorf("list gh yaml under %s: %w", ghRoot, err)
	}

	var issues []checkutil.Issue
	for _, file := range files {
		issues = append(issues, checkFile(file)...)
	}

	return &CheckResult{Issues: issues}, nil
}

func checkFile(file string) []checkutil.Issue {
	raw, err := os.ReadFile(file)
	if err != nil {
		return []checkutil.Issue{{
			File:     file,
			Severity: checkutil.SeverityError,
			Message:  fmt.Sprintf("read error: %v", err),
		}}
	}
	if strings.TrimSpace(string(raw)) == "" {
		return nil
	}

	var sections []sectionDTO
	if err := yaml.Unmarshal(raw, &sections); err != nil {
		return []checkutil.Issue{{
			File:     file,
			Severity: checkutil.SeverityError,
			Message:  fmt.Sprintf("YAML parse error (top level must be list of mappings): %v", err),
		}}
	}

	var issues []checkutil.Issue
	for i, section := range sections {
		issues = append(issues, checkSection(file, i, section)...)
	}

	return issues
}

func checkSection(file string, index int, section sectionDTO) []checkutil.Issue {
	rules := sectionRules{
		Type:   section.Type,
		Topics: make([]struct{}, len(section.Topics)),
	}
	errs := validateStruct(&rules)
	prefix := sectionPrefix(section, index)

	var issues []checkutil.Issue
	// Fixed field order so Issues order is deterministic (map range is not).
	for _, field := range []string{"Type", "Topics"} {
		fieldErrs, ok := errs[field]
		if !ok {
			continue
		}
		for _, rule := range sortedKeys(fieldErrs) {
			msg := fieldErrs[rule]
			message := msg
			switch {
			case field == "Topics" && (rule == "maxLen" || rule == "max_len"):
				n := len(section.Topics)
				message = fmt.Sprintf("too many topics (%d > %d)", n, MaxTopicsPerSection)
			case field == "Topics" && (rule == "required" || rule == "minLen" || rule == "min_len"):
				message = "topics is required (at least 1)"
			case field == "Type" && rule == "required":
				message = "type is required"
			}
			issues = append(issues, checkutil.Issue{
				File:     file,
				Severity: checkutil.SeverityError,
				Message:  prefix + ": " + message,
			})
		}
	}

	// Still validate each topic even when section-level rules fail (e.g. missing type).
	for _, topic := range section.Topics {
		issues = append(issues, checkTopic(file, topic)...)
	}

	return issues
}

func checkTopic(file string, topic topicDTO) []checkutil.Issue {
	prefix := topicPrefix(topic)
	rules := topicRules{Topic: topic.Topic, Kind: topic.Kind}
	errs := validateStruct(&rules)

	var issues []checkutil.Issue
	for _, field := range []string{"Topic", "Kind"} {
		fieldErrs, ok := errs[field]
		if !ok {
			continue
		}
		for _, rule := range sortedKeys(fieldErrs) {
			msg := fieldErrs[rule]
			message := msg
			if field == "Kind" && rule == "in" {
				// Inject the illegal value; gookit message placeholders cannot do this reliably.
				message = fmt.Sprintf("invalid kind %q (allowed: %s)", strings.TrimSpace(topic.Kind), AllowedKindsCSV)
			}
			issues = append(issues, checkutil.Issue{
				File:     file,
				Severity: checkutil.SeverityError,
				Message:  prefix + ": " + message,
			})
		}
	}

	// Sole manual gate: gookit treats nil nested pointers as empty structs and runs child required.
	// mdscc is optional for every kind; if present, all five fields are required.
	if topic.Mdscc != nil {
		issues = append(issues, checkTopicMdscc(file, prefix, topic.Mdscc)...)
	}

	return issues
}

func checkTopicMdscc(file, prefix string, m *mdsccDTO) []checkutil.Issue {
	rules := mdsccRules{
		Meta:   m.Meta,
		Derive: m.Derive,
		Sol:    m.Sol,
		Cost:   m.Cost,
		Case:   m.Case,
	}
	errs := validateStruct(&rules)

	var issues []checkutil.Issue
	for _, field := range []string{"Meta", "Derive", "Sol", "Cost", "Case"} {
		fieldErrs, ok := errs[field]
		if !ok {
			continue
		}
		for _, rule := range sortedKeys(fieldErrs) {
			issues = append(issues, checkutil.Issue{
				File:     file,
				Severity: checkutil.SeverityError,
				Message:  prefix + ": " + fieldErrs[rule],
			})
		}
	}

	return issues
}

func sortedKeys(m validate.MS) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	return keys
}

func validateStruct(s any) validate.Errors {
	v := validate.Struct(s)
	v.StopOnError = false

	return v.ValidateE()
}

func sectionPrefix(section sectionDTO, index int) string {
	if name := strings.TrimSpace(section.Type); name != "" {
		return fmt.Sprintf("section type %q", name)
	}

	return fmt.Sprintf("section[%d]", index)
}

func topicPrefix(topic topicDTO) string {
	if name := strings.TrimSpace(topic.Topic); name != "" {
		return fmt.Sprintf("topic %q", name)
	}

	return "topic entry"
}
