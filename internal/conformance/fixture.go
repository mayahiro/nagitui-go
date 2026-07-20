// Package conformance contains private helpers for shared fixture tests
package conformance

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"unicode/utf8"
)

// ErrNoFixtureRoot indicates that NAGI_FIXTURES is not configured
var ErrNoFixtureRoot = errors.New("NAGI_FIXTURES is not configured")

// Record is one parsed fixture case
type Record struct {
	ID     string
	Fields map[string]string
}

// Field returns a required raw field value
func (r Record) Field(name string) string {
	value, ok := r.Fields[name]
	if !ok {
		panic(fmt.Sprintf("case %s has no %s field", r.ID, name))
	}
	return value
}

// Bytes decodes fixture escapes without requiring valid UTF-8
func (r Record) Bytes(name string) []byte {
	value, err := Decode(r.Field(name))
	if err != nil {
		panic(fmt.Sprintf("case %s has invalid %s field: %v", r.ID, name, err))
	}
	return value
}

// Text decodes a field and requires valid UTF-8
func (r Record) Text(name string) string {
	value := r.Bytes(name)
	if !utf8.Valid(value) {
		panic(fmt.Sprintf("case %s has non-UTF-8 %s field", r.ID, name))
	}
	return string(value)
}

// Load parses one shared fixture file
func Load(relative, suite string, allowedFields ...string) ([]Record, error) {
	root := os.Getenv("NAGI_FIXTURES")
	if root == "" {
		return nil, ErrNoFixtureRoot
	}
	path := filepath.Join(root, filepath.FromSlash(relative))
	input, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	return parse(path, input, suite, allowedFields)
}

func parse(path string, input []byte, suite string, allowedFields []string) ([]Record, error) {
	allowed := make(map[string]struct{}, len(allowedFields))
	for _, field := range allowedFields {
		allowed[field] = struct{}{}
	}
	headerSeen := false
	caseIDs := make(map[string]struct{})
	var records []Record
	scanner := bufio.NewScanner(bytes.NewReader(input))
	for lineNumber := 1; scanner.Scan(); lineNumber++ {
		line := scanner.Text()
		if strings.TrimSpace(line) == "" || strings.HasPrefix(strings.TrimLeft(line, " \t"), "#") {
			continue
		}
		if !headerSeen {
			if line != "nagi-fixture-v1\t"+suite {
				return nil, fixtureError(path, lineNumber, "unsupported or mismatched header")
			}
			headerSeen = true
			continue
		}

		parts := strings.Split(line, "\t")
		id := parts[0]
		if !validCaseID(id) {
			return nil, fixtureError(path, lineNumber, "invalid case identifier")
		}
		if _, duplicate := caseIDs[id]; duplicate {
			return nil, fixtureError(path, lineNumber, "duplicate case identifier")
		}
		caseIDs[id] = struct{}{}
		fields := make(map[string]string, len(parts)-1)
		for _, part := range parts[1:] {
			name, value, ok := strings.Cut(part, "=")
			if !ok {
				return nil, fixtureError(path, lineNumber, "field has no equals sign")
			}
			if _, known := allowed[name]; !known {
				return nil, fixtureError(path, lineNumber, "unknown field")
			}
			if _, duplicate := fields[name]; duplicate {
				return nil, fixtureError(path, lineNumber, "duplicate field")
			}
			fields[name] = value
		}
		if len(fields) != len(allowed) {
			return nil, fixtureError(path, lineNumber, "missing field")
		}
		records = append(records, Record{ID: id, Fields: fields})
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("scan %s: %w", path, err)
	}
	if !headerSeen {
		return nil, fixtureError(path, 1, "missing header")
	}
	return records, nil
}

func validCaseID(id string) bool {
	if id == "" {
		return false
	}
	for _, character := range []byte(id) {
		if (character < 'a' || character > 'z') && (character < '0' || character > '9') && character != '-' && character != '_' {
			return false
		}
	}
	return true
}

func fixtureError(path string, line int, reason string) error {
	return fmt.Errorf("%s:%d: %s", path, line, reason)
}

// Decode decodes canonical fixture escapes
func Decode(value string) ([]byte, error) {
	var output []byte
	for index := 0; index < len(value); {
		if value[index] != '\\' {
			_, size := utf8.DecodeRuneInString(value[index:])
			output = append(output, value[index:index+size]...)
			index += size
			continue
		}
		index++
		if index == len(value) {
			return nil, errors.New("incomplete escape")
		}
		switch value[index] {
		case '\\':
			output = append(output, '\\')
			index++
		case 't':
			output = append(output, '\t')
			index++
		case 'n':
			output = append(output, '\n')
			index++
		case 'r':
			output = append(output, '\r')
			index++
		case 'x':
			if index+2 >= len(value) {
				return nil, errors.New("incomplete byte escape")
			}
			high, err := hex(value[index+1])
			if err != nil {
				return nil, err
			}
			low, err := hex(value[index+2])
			if err != nil {
				return nil, err
			}
			output = append(output, high<<4|low)
			index += 3
		case 'u':
			if index+1 >= len(value) || value[index+1] != '{' {
				return nil, errors.New("Unicode escape has no opening brace")
			}
			end := strings.IndexByte(value[index+2:], '}')
			if end < 0 {
				return nil, errors.New("incomplete Unicode escape")
			}
			digits := value[index+2 : index+2+end]
			if digits == "" || len(digits) > 6 {
				return nil, errors.New("empty or overlong Unicode escape")
			}
			for _, digit := range []byte(digits) {
				if _, err := hex(digit); err != nil {
					return nil, err
				}
			}
			scalar, err := strconv.ParseUint(digits, 16, 32)
			if err != nil || !utf8.ValidRune(rune(scalar)) {
				return nil, errors.New("invalid Unicode scalar value")
			}
			output = utf8.AppendRune(output, rune(scalar))
			index += end + 3
		default:
			return nil, fmt.Errorf("unknown escape %c", value[index])
		}
	}
	return output, nil
}

func hex(digit byte) (byte, error) {
	switch {
	case digit >= '0' && digit <= '9':
		return digit - '0', nil
	case digit >= 'A' && digit <= 'F':
		return digit - 'A' + 10, nil
	default:
		return 0, errors.New("non-canonical hexadecimal digit")
	}
}
