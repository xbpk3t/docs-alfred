package session

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

const (
	jsonlScanBufInit         = 256 * 1024
	defaultJSONLScanMaxToken = 16 * 1024 * 1024 // 16 MiB; real CC tool_result lines can exceed 1 MiB
)

// jsonlScanMaxToken is the Scanner max token size. Overridable in tests.
var jsonlScanMaxToken = defaultJSONLScanMaxToken

// scanJSONLLines opens path and invokes fn for each non-empty trimmed line.
// lineNum is 1-based and counts all scanned lines, including skipped blanks.
// Scanner errors (including bufio.ErrTooLong) are returned via scanErrFmt.
func scanJSONLLines(path, openErrFmt, scanErrFmt string, fn func(lineNum int, line string) error) error {
	f, err := os.Open(path)
	if err != nil {
		return fmt.Errorf(openErrFmt, err)
	}
	defer func() { _ = f.Close() }()

	// Effective max token is max(maxToken, cap(buf)); keep init cap <= maxToken
	// so tests can lower jsonlScanMaxToken without the 256KiB init raising the limit.
	initCap := jsonlScanBufInit
	if initCap > jsonlScanMaxToken {
		initCap = jsonlScanMaxToken
	}

	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, initCap), jsonlScanMaxToken)

	lineNum := 0
	for scanner.Scan() {
		lineNum++
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		if err := fn(lineNum, line); err != nil {
			return err
		}
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf(scanErrFmt, err)
	}

	return nil
}
