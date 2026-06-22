package internal

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// ChainRecord represents a session in the chain.
type ChainRecord struct {
	SessionID      string  `json:"sessionId"`
	PrevSessionID  *string `json:"prevSessionId"`
	StartedAt      string  `json:"startedAt"`
	EndedAt        *string `json:"endedAt"`
	Display        string  `json:"display"`
	TranscriptPath string  `json:"transcriptPath"`
	ParentUUID     string  `json:"parentUuid,omitempty"`
	IsSidechain    bool    `json:"isSidechain,omitempty"`
}

// WalkSessionChain walks the session chain based on parentUuid relationships.
// This finds the main session and all its sub-agents (sidechains).
// If sessionIDOverride is non-empty, it is used directly instead of auto-detection.
func WalkSessionChain(sessionIDOverride string) ([]ChainRecord, error) {
	projectDir := getProjectDir()

	// Get session ID
	sessionID, err := getSessionID(projectDir, sessionIDOverride)
	if err != nil {
		return nil, fmt.Errorf("get session ID: %w", err)
	}

	// Get transcript path
	pathKey := strings.ReplaceAll(projectDir, "/", "-")
	sessionDir := filepath.Join(os.Getenv("HOME"), ".claude", "projects", pathKey)
	transcriptPath := filepath.Join(sessionDir, sessionID+".jsonl")

	// Check if this session is a sidechain
	isSidechain, parentUUID := checkIfSidechain(transcriptPath)

	// If this is a sidechain, find the main session
	if isSidechain && parentUUID != "" {
		// Find the main session
		mainSessionPath := findSessionByID(sessionDir, parentUUID)
		if mainSessionPath != "" {
			// Use the main session as the starting point
			sessionID = parentUUID
			transcriptPath = mainSessionPath
		}
	}

	// Build the chain: main session + all its sidechains
	chain, err := buildChainFromMainSession(sessionDir, sessionID, transcriptPath)
	if err != nil {
		return nil, fmt.Errorf("build chain: %w", err)
	}

	// Save chain to file
	if err := saveChainToFile(projectDir, chain); err != nil {
		return nil, fmt.Errorf("save chain: %w", err)
	}

	return chain, nil
}

// checkIfSidechain checks if a session is a sidechain by looking at the first line.
//
func checkIfSidechain(transcriptPath string) (bool, string) {
	file, err := os.Open(filepath.Clean(transcriptPath))
	if err != nil {
		return false, ""
	}
	defer func() { _ = file.Close() }()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		var obj map[string]any
		if err := json.Unmarshal([]byte(line), &obj); err != nil {
			continue
		}

		// Check if this is a sidechain
		if isSidechain, ok := obj["isSidechain"].(bool); ok && isSidechain {
			// Get parentUuid
			if parentUUID, ok := obj["parentUuid"].(string); ok && parentUUID != "" {
				return true, parentUUID
			}
		}

		// Only check the first non-empty line
		break
	}

	return false, ""
}

// findSessionByID finds a session file by its ID.
func findSessionByID(sessionDir, sessionID string) string {
	// Check main session directory
	mainPath := filepath.Join(sessionDir, sessionID+".jsonl")
	if _, err := os.Stat(filepath.Clean(mainPath)); err == nil {
		return mainPath
	}

	// Check sidechains directory
	sidechainPath := filepath.Join(sessionDir, "sidechains", sessionID+".jsonl")
	if _, err := os.Stat(filepath.Clean(sidechainPath)); err == nil {
		return sidechainPath
	}

	return ""
}

// buildChainFromMainSession builds a chain starting from a main session.
func buildChainFromMainSession(sessionDir, mainSessionID, mainTranscriptPath string) ([]ChainRecord, error) {
	var chain []ChainRecord

	// Add main session
	mainRecord := createChainRecord(mainSessionID, mainTranscriptPath, nil, false)
	chain = append(chain, mainRecord)

	// Find all sidechains of this main session
	sidechains, err := findSidechainsOf(sessionDir, mainSessionID)
	if err != nil {
		return nil, fmt.Errorf("find sidechains: %w", err)
	}

	// Sort sidechains by timestamp
	sort.Slice(sidechains, func(i, j int) bool {
		return sidechains[i].StartedAt < sidechains[j].StartedAt
	})

	// Add sidechains to chain
	for i, sc := range sidechains {
		prevSessionID := mainSessionID
		if i > 0 {
			prevSessionID = sidechains[i-1].SessionID
		}
		sc.PrevSessionID = &prevSessionID
		chain = append(chain, sc)
	}

	return chain, nil
}

// findSidechainsOf finds all sidechains of a given session.
func findSidechainsOf(sessionDir, parentSessionID string) ([]ChainRecord, error) {
	sidechainDir := filepath.Join(sessionDir, "sidechains")
	if _, err := os.Stat(filepath.Clean(sidechainDir)); os.IsNotExist(err) {
		return nil, nil
	}

	entries, err := os.ReadDir(sidechainDir)
	if err != nil {
		return nil, fmt.Errorf("read sidechain dir: %w", err)
	}

	var sidechains []ChainRecord

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".jsonl") {
			continue
		}

		sessionID := strings.TrimSuffix(entry.Name(), ".jsonl")
		transcriptPath := filepath.Join(sidechainDir, entry.Name())

		// Check if this sidechain belongs to the parent session
		isSidechain, parentUUID := checkIfSidechain(transcriptPath)
		if isSidechain && parentUUID == parentSessionID {
			record := createChainRecord(sessionID, transcriptPath, nil, true)
			record.ParentUUID = parentUUID
			sidechains = append(sidechains, record)
		}
	}

	return sidechains, nil
}

// createChainRecord creates a ChainRecord from a session file.
func createChainRecord(sessionID, transcriptPath string, prevSessionID *string, isSidechain bool) ChainRecord {
	// Extract display text
	display := extractDisplay(transcriptPath)
	if display == "" {
		display = sessionID
	}

	// Extract startedAt
	startedAt := extractStartedAt(transcriptPath)
	if startedAt == "" {
		startedAt = time.Now().Format(time.RFC3339)
	}

	return ChainRecord{
		SessionID:      sessionID,
		PrevSessionID:  prevSessionID,
		StartedAt:      startedAt,
		EndedAt:        nil,
		Display:        display,
		TranscriptPath: transcriptPath,
		IsSidechain:    isSidechain,
	}
}

// getProjectDir returns the project directory.
func getProjectDir() string {
	if projectDir := os.Getenv("CLAUDE_PROJECT_DIR"); projectDir != "" {
		return projectDir
	}

	// Fallback: current working directory
	cwd, err := os.Getwd()
	if err != nil {
		return "."
	}

	return cwd
}

// getSessionID returns the session ID.
// Priority: sessionIDOverride > CLAUDE_CODE_SESSION_ID env var > error.
func getSessionID(projectDir, sessionIDOverride string) (string, error) {
	// 1. Explicit override (e.g. --session flag)
	if sessionIDOverride != "" {
		return sessionIDOverride, nil
	}

	// 2. Environment variable set by Claude Code
	if sessionID := os.Getenv("CLAUDE_CODE_SESSION_ID"); sessionID != "" {
		return sessionID, nil
	}

	// 3. No session ID available — report all sessions and exit
	pathKey := strings.ReplaceAll(projectDir, "/", "-")
	sessionDir := filepath.Join(os.Getenv("HOME"), ".claude", "projects", pathKey)

	entries, err := os.ReadDir(sessionDir)
	if err != nil {
		return "", fmt.Errorf("read session dir: %w", err)
	}

	var sessions []string
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".jsonl") {
			continue
		}
		sessions = append(sessions, strings.TrimSuffix(entry.Name(), ".jsonl"))
	}

	if len(sessions) == 0 {
		return "", fmt.Errorf("no session files found in %s", sessionDir)
	}

	return "", fmt.Errorf("no session ID specified (CLAUDE_CODE_SESSION_ID not set); use --session <id> to specify one; available sessions: %s", strings.Join(sessions, ", "))
}

// extractDisplay extracts the first user message as display text (max 80 chars).
func extractDisplay(transcriptPath string) string {
	file, err := os.Open(filepath.Clean(transcriptPath))
	if err != nil {
		return ""
	}
	defer func() { _ = file.Close() }()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		var obj map[string]any
		if err := json.Unmarshal([]byte(line), &obj); err != nil {
			continue
		}

		// Check if this is a user message
		if obj["type"] != roleUser {
			continue
		}

		// Get message content
		message, ok := obj["message"].(map[string]any)
		if !ok {
			continue
		}

		content, ok := message["content"].(string)
		if !ok {
			continue
		}

		// Clean up content
		content = strings.TrimSpace(content)
		content = strings.ReplaceAll(content, "\n", " ")
		content = strings.ReplaceAll(content, "\r", " ")
		content = strings.ReplaceAll(content, "\t", " ")

		// Collapse multiple spaces
		for strings.Contains(content, "  ") {
			content = strings.ReplaceAll(content, "  ", " ")
		}

		// Truncate to 80 chars
		if len(content) > 80 {
			content = content[:80]
		}

		return content
	}

	return ""
}

// extractStartedAt extracts the timestamp from the first timestamped entry.
func extractStartedAt(transcriptPath string) string {
	file, err := os.Open(filepath.Clean(transcriptPath))
	if err != nil {
		return ""
	}
	defer func() { _ = file.Close() }()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		var obj map[string]any
		if err := json.Unmarshal([]byte(line), &obj); err != nil {
			continue
		}

		// Check if this entry has a timestamp
		if timestamp, ok := obj["timestamp"].(string); ok && timestamp != "" {
			return timestamp
		}
	}

	return ""
}

// saveChainToFile saves the chain to a JSONL file.
func saveChainToFile(projectDir string, chain []ChainRecord) error {
	chainDir := filepath.Join(projectDir, ".claude", "session-trail")
	if err := os.MkdirAll(chainDir, 0750); err != nil {
		return fmt.Errorf("create chain dir: %w", err)
	}
	chainFile := filepath.Join(chainDir, "chain.jsonl")

	file, err := os.Create(filepath.Clean(chainFile))
	if err != nil {
		return err
	}
	defer func() { _ = file.Close() }()

	writer := bufio.NewWriter(file)
	defer func() { _ = writer.Flush() }()

	for _, record := range chain {
		data, err := json.Marshal(record)
		if err != nil {
			return err
		}
		if _, err := writer.Write(data); err != nil {
			return err
		}
		if _, err := writer.WriteString("\n"); err != nil {
			return err
		}
	}

	return nil
}
