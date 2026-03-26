package satellite

import (
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"
)

// TLERecord represents one named two-line element set.
type TLERecord struct {
	Name  string
	Line1 string
	Line2 string
}

// NewTLERecord validates the raw TLE lines and applies the catalog-number fallback
// when the source does not provide a display name.
func NewTLERecord(name, line1, line2 string) (TLERecord, error) {
	if err := validateTLELines(line1, line2); err != nil {
		return TLERecord{}, err
	}

	if name == "" {
		name = strings.TrimSpace(line1[2:7])
	}

	return TLERecord{
		Name:  name,
		Line1: line1,
		Line2: line2,
	}, nil
}

// scanNextNonEmptyLine returns the next non-blank line from the scanner.
func scanNextNonEmptyLine(scan func() bool, text func() string, scanErr func() error) (string, error) {
	for scan() {
		line := text()
		if strings.TrimSpace(line) != "" {
			return line, nil
		}
	}

	if err := scanErr(); err != nil {
		return "", err
	}
	return "", io.EOF
}

// readTLERecord consumes either a 2-line or 3-line TLE record from the scanner.
func readTLERecord(scan func() bool, text func() string, scanErr func() error) (TLERecord, error) {
	firstLine, err := scanNextNonEmptyLine(scan, text, scanErr)
	if err != nil {
		return TLERecord{}, err
	}

	name := ""
	line1 := firstLine
	if !strings.HasPrefix(firstLine, "1") {
		name = strings.TrimSpace(firstLine)
		line1, err = scanNextNonEmptyLine(scan, text, scanErr)
		if err != nil {
			if errors.Is(err, io.EOF) {
				return TLERecord{}, errors.New(errCannotParse)
			}
			return TLERecord{}, err
		}
	}

	if !strings.HasPrefix(line1, "1") {
		return TLERecord{}, errors.New(errCannotParse)
	}

	line2, err := scanNextNonEmptyLine(scan, text, scanErr)
	if err != nil {
		if errors.Is(err, io.EOF) {
			return TLERecord{}, errors.New(errCannotParse)
		}
		return TLERecord{}, err
	}
	if !strings.HasPrefix(line2, "2") {
		return TLERecord{}, errors.New(errCannotParse)
	}

	return NewTLERecord(name, line1, line2)
}

// validateTLELines parses the fields that the upstream SGP4 package would otherwise
// parse with fatal logging, so malformed input can be returned as an ordinary error.
func validateTLELines(line1, line2 string) error {
	if len(line1) < 69 || len(line2) < 69 {
		return fmt.Errorf("invalid TLE length")
	}

	if !strings.HasPrefix(line1, "1") || !strings.HasPrefix(line2, "2") {
		return errors.New(errCannotParse)
	}

	if _, err := strconv.Atoi(strings.TrimSpace(line1[2:7])); err != nil {
		return fmt.Errorf("invalid TLE satellite number: %w", err)
	}
	if _, err := strconv.Atoi(strings.TrimSpace(line1[18:20])); err != nil {
		return fmt.Errorf("invalid TLE epoch year: %w", err)
	}
	if _, err := strconv.ParseFloat(strings.TrimSpace(line1[20:32]), 64); err != nil {
		return fmt.Errorf("invalid TLE epoch day: %w", err)
	}
	if _, err := strconv.ParseFloat(strings.TrimSpace(line2[8:16]), 64); err != nil {
		return fmt.Errorf("invalid TLE inclination: %w", err)
	}
	if _, err := strconv.ParseFloat(strings.TrimSpace(line2[17:25]), 64); err != nil {
		return fmt.Errorf("invalid TLE RAAN: %w", err)
	}
	if _, err := strconv.ParseFloat("0."+strings.TrimSpace(line2[26:33]), 64); err != nil {
		return fmt.Errorf("invalid TLE eccentricity: %w", err)
	}
	if _, err := strconv.ParseFloat(strings.TrimSpace(line2[34:42]), 64); err != nil {
		return fmt.Errorf("invalid TLE argument of perigee: %w", err)
	}
	if _, err := strconv.ParseFloat(strings.TrimSpace(line2[43:51]), 64); err != nil {
		return fmt.Errorf("invalid TLE mean anomaly: %w", err)
	}
	if _, err := strconv.ParseFloat(strings.TrimSpace(line2[52:63]), 64); err != nil {
		return fmt.Errorf("invalid TLE mean motion: %w", err)
	}
	if _, err := parseSignedMantissaExponent(line1[44:45], line1[45:50], line1[50:52]); err != nil {
		return fmt.Errorf("invalid TLE nddot: %w", err)
	}
	if _, err := parseSignedMantissaExponent(line1[53:54], line1[54:59], line1[59:61]); err != nil {
		return fmt.Errorf("invalid TLE bstar: %w", err)
	}
	if _, err := strconv.ParseFloat(strings.ReplaceAll(line1[33:43], " ", ""), 64); err != nil {
		return fmt.Errorf("invalid TLE ndot: %w", err)
	}

	return nil
}

// parseSignedMantissaExponent decodes TLE compact exponent fields such as BSTAR.
func parseSignedMantissaExponent(sign, mantissa, exponent string) (float64, error) {
	return strconv.ParseFloat(strings.ReplaceAll(sign+"."+mantissa+"e"+exponent, " ", ""), 64)
}

// Epoch parses the TLE epoch from line 1.
func (r TLERecord) Epoch() (time.Time, error) {
	return ParseTLEEpoch(r.Line1)
}

// ParseTLEEpoch converts the fixed-width epoch field from TLE line 1 into UTC time.
func ParseTLEEpoch(line1 string) (time.Time, error) {
	if len(line1) < 32 {
		return time.Time{}, fmt.Errorf("invalid TLE length")
	}

	yearValue, err := strconv.Atoi(strings.TrimSpace(line1[18:20]))
	if err != nil {
		return time.Time{}, fmt.Errorf("invalid TLE epoch year: %w", err)
	}
	dayValue, err := strconv.ParseFloat(strings.TrimSpace(line1[20:32]), 64)
	if err != nil {
		return time.Time{}, fmt.Errorf("invalid TLE epoch day: %w", err)
	}

	year := 1900 + yearValue
	if yearValue < 57 {
		year = 2000 + yearValue
	}

	startOfYear := time.Date(year, time.January, 1, 0, 0, 0, 0, time.UTC)
	dayOffset := (dayValue - 1) * 24 * float64(time.Hour)
	return startOfYear.Add(time.Duration(dayOffset)), nil
}
