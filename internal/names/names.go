// Package names validates identifiers that become directory names.
package names

import (
	"fmt"
	"regexp"
	"strings"
	"time"
)

var (
	idRe      = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,63}$`)
	groupRe   = regexp.MustCompile(`^[a-z0-9][a-z0-9_-]{0,31}$`)
	extNameRe = regexp.MustCompile(`^[a-z0-9][a-z0-9_-]{0,63}$`)
	versionRe = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._+-]{0,63}$`)
	localeRe  = regexp.MustCompile(`^[A-Za-z]{2,3}([_-][A-Za-z0-9]{2,8}){0,3}$`)
)

// ValidID reports whether s is a safe profile or template id.
func ValidID(s string) error {
	if !idRe.MatchString(s) || strings.HasSuffix(s, ".") {
		return fmt.Errorf("invalid id %q (use 1–64 letters, digits, '.', '_' or '-', starting with a letter or digit and not ending with '.')", s)
	}
	if isReservedWindows(s) {
		return fmt.Errorf("invalid id %q (reserved device name on Windows)", s)
	}
	return nil
}

// ValidGroup reports whether s is a safe extension-group name.
func ValidGroup(s string) error {
	if !groupRe.MatchString(s) {
		return fmt.Errorf("invalid group name %q (lowercase letters, digits, '_' or '-', starting with a letter or digit, at most 32 characters)", s)
	}
	if isReservedWindows(s) {
		return fmt.Errorf("invalid group name %q (reserved device name on Windows)", s)
	}
	return nil
}

// ValidExtName reports whether s is a safe extension store name.
func ValidExtName(s string) error {
	if !extNameRe.MatchString(s) {
		return fmt.Errorf("invalid extension name %q (lowercase letters, digits, '_' or '-', starting with a letter or digit, at most 64 characters)", s)
	}
	if isReservedWindows(s) {
		return fmt.Errorf("invalid extension name %q (reserved device name on Windows)", s)
	}
	return nil
}

// ValidVersion reports whether s can be used as an extension version directory.
func ValidVersion(s string) error {
	if !versionRe.MatchString(s) {
		return fmt.Errorf("invalid extension version %q", s)
	}
	return nil
}

// ValidEmail is a practical check, not a full RFC parser.
func ValidEmail(s string) error {
	if s == "" || len(s) > 254 || strings.ContainsAny(s, " \t\r\n/\\") {
		return fmt.Errorf("invalid email %q", s)
	}
	local, domain, ok := strings.Cut(s, "@")
	if !ok || local == "" || domain == "" || strings.Contains(domain, "@") || !strings.Contains(domain, ".") {
		return fmt.Errorf("invalid email %q", s)
	}
	return nil
}

// ValidLocale checks a BCP 47-ish tag such as en-US. Empty is not valid here.
func ValidLocale(s string) error {
	if !localeRe.MatchString(s) {
		return fmt.Errorf("invalid locale %q (examples: en-US, en, pt-BR)", s)
	}
	return nil
}

// ValidTimezone checks an IANA time zone. Empty means the host zone and is valid.
func ValidTimezone(s string) error {
	if s == "" {
		return nil
	}
	if _, err := time.LoadLocation(s); err != nil {
		return fmt.Errorf("unknown time zone %q", s)
	}
	return nil
}

// ParseExtRef splits "name" or "name@version".
func ParseExtRef(s string) (name, version string, pinned bool, err error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return "", "", false, fmt.Errorf("extension reference is empty")
	}
	name, version, pinned = strings.Cut(s, "@")
	if err := ValidExtName(name); err != nil {
		return "", "", false, err
	}
	if pinned {
		if err := ValidVersion(version); err != nil {
			return "", "", false, fmt.Errorf("extension %q: %w", s, err)
		}
	}
	return name, version, pinned, nil
}

func isReservedWindows(s string) bool {
	base := s
	if i := strings.IndexByte(s, '.'); i >= 0 {
		base = s[:i]
	}
	switch strings.ToLower(base) {
	case "con", "prn", "aux", "nul",
		"com1", "com2", "com3", "com4", "com5", "com6", "com7", "com8", "com9",
		"lpt1", "lpt2", "lpt3", "lpt4", "lpt5", "lpt6", "lpt7", "lpt8", "lpt9":
		return true
	default:
		return false
	}
}
