package names

import (
	"fmt"
	"strconv"
	"strings"
	"unicode"
)

// SlugID turns a person's name into a profile id.
// "Mom" becomes "mom", "Mary Jane" becomes "mary-jane".
func SlugID(name string) (string, error) {
	var b strings.Builder
	prevHyphen := false
	for _, r := range strings.TrimSpace(name) {
		switch {
		case r >= 'A' && r <= 'Z':
			b.WriteRune(r - 'A' + 'a')
			prevHyphen = false
		case r >= 'a' && r <= 'z' || r >= '0' && r <= '9':
			b.WriteRune(r)
			prevHyphen = false
		default:
			if unicode.IsLetter(r) || unicode.IsDigit(r) {
				continue
			}
			if !prevHyphen && b.Len() > 0 {
				b.WriteByte('-')
				prevHyphen = true
			}
		}
	}
	slug := strings.Trim(b.String(), "-")
	if len(slug) > 64 {
		slug = strings.Trim(slug[:64], "-")
	}
	if err := ValidID(slug); err != nil {
		return "", fmt.Errorf("use a name with a letter or a number")
	}
	return slug, nil
}

// UniqueID returns base, or base-2, base-3, … until taken reports free.
func UniqueID(base string, taken func(string) bool) (string, error) {
	if err := ValidID(base); err != nil {
		return "", err
	}
	if taken == nil {
		taken = func(string) bool { return false }
	}
	if !taken(base) {
		return base, nil
	}
	for n := 2; n < 10000; n++ {
		suffix := "-" + strconv.Itoa(n)
		root := base
		if len(root)+len(suffix) > 64 {
			cut := 64 - len(suffix)
			if cut < 1 {
				return "", fmt.Errorf("could not find a free id")
			}
			root = strings.TrimRight(root[:cut], "-.")
		}
		id := root + suffix
		if ValidID(id) == nil && !taken(id) {
			return id, nil
		}
	}
	return "", fmt.Errorf("could not find a free id for %q", base)
}
