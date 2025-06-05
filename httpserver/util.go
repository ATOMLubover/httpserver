// some useful utility functions
package httpserver

import (
	"fmt"
	"path"
	"path/filepath"
	"regexp"
	"strings"
)

func CheckValidPattern(uri string) bool {
	pattern := `^[a-zA-Z0-9_/:*]+$`
	matched, _ := regexp.MatchString(pattern, uri)
	return matched
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
