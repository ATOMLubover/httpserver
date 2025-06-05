// some useful utility functions
package httpserver

import (
	"errors"
	"fmt"
	"path"
	"path/filepath"
	"regexp"
	"strings"
	"unicode"
)

func _CheckPatternCharactors(pattern string) bool {
	regexPattern := `^[a-zA-Z0-9_/:*]+$`
	matched, err := regexp.MatchString(regexPattern, pattern)
	return (err != nil) || matched
}

// validate colon wildcard rules
func _ValidateColonWildcard(part string) error {
	// format should be like ":name"
	if len(part) == 1 {
		return errors.New("colon wildcard must be named")
	}
	// colon should not be repeated
	if strings.Count(part, ":") > 1 {
		return errors.New("too many ':' in part: " + part)
	}
	// wildcard naming should be valid
	if !_CheckColonNameValid(strings.TrimPrefix(part, ":")) {
		return errors.New("invalid wildcard name: " + part)
	}

	return nil
}

// validate asterisk wildcard rules
func _ValidateAsteriskWildcard(part string, remainingPartsNum int) error {
	// make sure asterisk is last part
	if remainingPartsNum >= 1 {
		return errors.New("asterisk must be last part")
	}

	// validate naming
	name := strings.TrimPrefix(part, "*")
	if !_CheckAsteriskNameValid(name) {
		return errors.New("invalid asterisk name: " + name)
	}

	return nil
}

// check colon naming validation
func _CheckColonNameValid(name string) bool {
	if name == "" {
		return false
	}

	// first char must be letter or _
	if !(unicode.IsLetter(rune(name[0])) || name[0] == '_') {
		return false
	}

	for _, r := range name {
		if !(unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_') {
			return false
		}
	}

	// return true after checking
	return true
}

// check asterisk naming validation
func _CheckAsteriskNameValid(name string) bool {
	// asterisk allows empty name
	if name == "" {
		return true
	}

	// first char must be letter or _
	if !(unicode.IsLetter(rune(name[0])) || name[0] == '_') {
		return false
	}

	for _, r := range name {
		if !(unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_') {
			return false
		}
	}

	// return true after checking
	return true
}

// transform a pattern into valid parts when setting route
func _TransformPatternIntoParts(pattern string) ([]string, error) {
	// trim slashes
	trimmed := strings.Trim(pattern, "/")
	if trimmed == "" {
		return []string{}, nil // return empty slice when at root
	}

	parts := strings.Split(trimmed, "/")
	result := make([]string, 0, len(parts))

	for i, part := range parts {
		// continue when part is empty
		if len(part) == 0 {
			continue
		}

		// check wildcard rules
		switch {
		case strings.HasPrefix(part, ":"):
			if err := _ValidateColonWildcard(part); err != nil {
				return nil, fmt.Errorf("%w in: %s", err, pattern)
			}

		case strings.HasPrefix(part, "*"):
			if err := _ValidateAsteriskWildcard(part, len(parts)-i-1); err != nil {
				return nil, fmt.Errorf("%w in: %s", err, pattern)
			}
			result = append(result, part)
			return result, nil // return immidiately when reaching asterisk
		}

		result = append(result, part)
	}
	return result, nil
}

func _TransformUriIntoParts(uri string) ([]string, error) {
	// cross-platform support
	cleanPath := filepath.Clean(uri)
	if !path.IsAbs(cleanPath) && strings.HasPrefix(cleanPath, "../") {
		return nil, fmt.Errorf("path traversal beyond root detected in uri: %s", uri)
	}

	parts := strings.Split(cleanPath, "/")
	result := make([]string, 0, len(parts))

	for _, part := range parts {
		// skip empty parts and current directory
		if part == "" || part == "." {
			continue
		}

		if part == ".." {
			if len(result) == 0 {
				return nil, fmt.Errorf("path traversal beyond root detected")
			}
			result = result[:len(result)-1] // 回退一级
		}

		// add part to result
		result = append(result, part)
	}

	return result, nil
}
