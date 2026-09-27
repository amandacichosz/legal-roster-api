package resumeparser

import (
	"archive/zip"
	"bytes"
	"encoding/hex"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"unicode"
)

const Version = "resumeparser-v1"

type Result struct {
	Text            string
	Summary         string
	Email           string
	Phone           string
	YearsExperience *int32
	Skills          []string
	PracticeAreas   []string
	Jurisdictions   []string
	Education       []string
	Certifications  []string
	Languages       []string
}

func Parse(filename string, contentType string, data []byte) (Result, error) {
	ext := strings.ToLower(filepath.Ext(filename))
	var text string
	var err error
	switch ext {
	case ".docx":
		text, err = extractDOCXText(data)
	case ".pdf":
		text, err = extractPDFText(data)
	case ".txt":
		text = string(data)
	case ".rtf":
		text = extractRTFText(data)
	case ".doc":
		err = errors.New("legacy .doc parsing is not supported yet")
	default:
		err = fmt.Errorf("unsupported resume type %q", ext)
	}
	if err != nil {
		return Result{}, err
	}

	text = normalizeText(text)
	if text == "" {
		return Result{}, errors.New("resume did not contain extractable text")
	}

	years := inferYearsExperience(text)
	result := Result{
		Text:            text,
		Summary:         summarize(text),
		Email:           firstMatch(text, emailPattern),
		Phone:           firstMatch(text, phonePattern),
		YearsExperience: years,
		Skills:          keywordMatches(text, legalSkills),
		PracticeAreas:   keywordMatches(text, practiceAreas),
		Jurisdictions:   keywordMatches(text, jurisdictions),
		Education:       educationMatches(text),
		Certifications:  keywordMatches(text, certifications),
		Languages:       keywordMatches(text, languages),
	}
	return result, nil
}

func extractDOCXText(data []byte) (string, error) {
	reader, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return "", err
	}
	for _, file := range reader.File {
		if file.Name != "word/document.xml" {
			continue
		}
		body, err := file.Open()
		if err != nil {
			return "", err
		}
		defer body.Close()
		return extractWordXMLText(body)
	}
	return "", errors.New("docx document body not found")
}

func extractWordXMLText(r io.Reader) (string, error) {
	decoder := xml.NewDecoder(r)
	var builder strings.Builder
	for {
		token, err := decoder.Token()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return "", err
		}
		switch element := token.(type) {
		case xml.StartElement:
			switch element.Name.Local {
			case "t":
				var value string
				if err := decoder.DecodeElement(&value, &element); err != nil {
					return "", err
				}
				builder.WriteString(value)
			case "tab", "br", "p":
				builder.WriteByte('\n')
			}
		}
	}
	return builder.String(), nil
}

func extractPDFText(data []byte) (string, error) {
	var values []string
	values = append(values, pdfLiteralStrings(data)...)
	values = append(values, pdfHexStrings(data)...)
	text := normalizeText(strings.Join(values, "\n"))
	if text == "" {
		return "", errors.New("pdf did not contain extractable text")
	}
	return text, nil
}

func extractRTFText(data []byte) string {
	value := string(data)
	var builder strings.Builder
	skipGroupDepth := 0
	groupDepth := 0
	for i := 0; i < len(value); i++ {
		char := value[i]
		if skipGroupDepth > 0 {
			switch char {
			case '{':
				groupDepth++
			case '}':
				groupDepth--
				if groupDepth < skipGroupDepth {
					skipGroupDepth = 0
				}
			}
			continue
		}
		switch char {
		case '\\':
			if i+1 >= len(value) {
				continue
			}
			i++
			controlStart := i
			if unicode.IsLetter(rune(value[i])) {
				for i < len(value) && unicode.IsLetter(rune(value[i])) {
					i++
				}
				controlWord := value[controlStart:i]
				for i < len(value) && (value[i] == '-' || unicode.IsDigit(rune(value[i]))) {
					i++
				}
				if i < len(value) && value[i] != ' ' {
					i--
				}
				switch controlWord {
				case "par", "line":
					builder.WriteByte('\n')
				case "tab":
					builder.WriteByte('\t')
				}
				continue
			}
			switch value[i] {
			case '\\', '{', '}':
				builder.WriteByte(value[i])
			case '*':
				skipGroupDepth = groupDepth
			}
		case '{':
			groupDepth++
		case '}':
			if groupDepth > 0 {
				groupDepth--
			}
		default:
			builder.WriteByte(char)
		}
	}
	return builder.String()
}

func pdfLiteralStrings(data []byte) []string {
	matches := regexp.MustCompile(`\((?:\\.|[^\\)]){2,}\)`).FindAll(data, -1)
	values := make([]string, 0, len(matches))
	for _, match := range matches {
		value := decodePDFLiteral(string(match[1 : len(match)-1]))
		if usefulText(value) {
			values = append(values, value)
		}
	}
	return values
}

func pdfHexStrings(data []byte) []string {
	matches := regexp.MustCompile(`<[0-9A-Fa-f\s]{4,}>`).FindAll(data, -1)
	values := make([]string, 0, len(matches))
	for _, match := range matches {
		raw := regexp.MustCompile(`\s+`).ReplaceAll(match[1:len(match)-1], nil)
		if len(raw)%2 == 1 {
			raw = append(raw, '0')
		}
		decoded := make([]byte, hex.DecodedLen(len(raw)))
		n, err := hex.Decode(decoded, raw)
		if err != nil {
			continue
		}
		value := string(decoded[:n])
		if usefulText(value) {
			values = append(values, value)
		}
	}
	return values
}

func decodePDFLiteral(value string) string {
	var builder strings.Builder
	escaped := false
	for _, r := range value {
		if escaped {
			switch r {
			case 'n', 'r':
				builder.WriteByte('\n')
			case 't':
				builder.WriteByte('\t')
			default:
				builder.WriteRune(r)
			}
			escaped = false
			continue
		}
		if r == '\\' {
			escaped = true
			continue
		}
		builder.WriteRune(r)
	}
	return builder.String()
}

func usefulText(value string) bool {
	value = strings.TrimSpace(value)
	if len(value) < 2 {
		return false
	}
	printable := 0
	letters := 0
	for _, r := range value {
		if unicode.IsPrint(r) || unicode.IsSpace(r) {
			printable++
		}
		if unicode.IsLetter(r) {
			letters++
		}
	}
	return printable >= len([]rune(value))*8/10 && letters > 0
}

func normalizeText(value string) string {
	value = strings.ReplaceAll(value, "\x00", "")
	lines := strings.Split(value, "\n")
	cleaned := make([]string, 0, len(lines))
	for _, line := range lines {
		line = strings.Join(strings.Fields(line), " ")
		if line != "" {
			cleaned = append(cleaned, line)
		}
	}
	return strings.TrimSpace(strings.Join(cleaned, "\n"))
}

func summarize(text string) string {
	lines := strings.Split(text, "\n")
	parts := make([]string, 0, 3)
	for _, line := range lines {
		if len(parts) == 3 {
			break
		}
		line = strings.TrimSpace(line)
		if line == "" || strings.Contains(line, "@") || phonePattern.MatchString(line) {
			continue
		}
		parts = append(parts, line)
	}
	summary := strings.Join(parts, " ")
	if len(summary) > 600 {
		summary = summary[:600]
	}
	return strings.TrimSpace(summary)
}

func firstMatch(text string, pattern *regexp.Regexp) string {
	match := pattern.FindString(text)
	return strings.TrimSpace(match)
}

func inferYearsExperience(text string) *int32 {
	pattern := regexp.MustCompile(`(?i)(\d{1,2})\+?\s*(?:years|yrs)\s+(?:of\s+)?(?:legal\s+)?experience`)
	matches := pattern.FindAllStringSubmatch(text, -1)
	var max int32
	for _, match := range matches {
		years, err := strconv.Atoi(match[1])
		if err == nil && int32(years) > max {
			max = int32(years)
		}
	}
	if max == 0 {
		return nil
	}
	return &max
}

func educationMatches(text string) []string {
	candidates := keywordMatches(text, educationKeywords)
	if len(candidates) > 0 {
		return candidates
	}
	return nil
}

func keywordMatches(text string, keywords []string) []string {
	lower := strings.ToLower(text)
	matches := make([]string, 0)
	for _, keyword := range keywords {
		if strings.Contains(lower, strings.ToLower(keyword)) {
			matches = append(matches, keyword)
		}
	}
	slices.Sort(matches)
	return slices.Compact(matches)
}

var (
	emailPattern = regexp.MustCompile(`[A-Za-z0-9._%+\-]+@[A-Za-z0-9.\-]+\.[A-Za-z]{2,}`)
	phonePattern = regexp.MustCompile(`(?:\+?1[\s.-]?)?(?:\(?\d{3}\)?[\s.-]?)\d{3}[\s.-]?\d{4}`)
)

var legalSkills = []string{
	"case management", "client intake", "contract review", "document review", "drafting", "e-filing",
	"legal research", "litigation support", "matter management", "trial preparation", "westlaw", "lexisnexis",
}

var practiceAreas = []string{
	"bankruptcy", "business law", "civil litigation", "criminal defense", "employment law", "estate planning",
	"family law", "immigration", "intellectual property", "personal injury", "probate", "real estate",
}

var jurisdictions = []string{
	"alabama", "alaska", "arizona", "arkansas", "california", "colorado", "connecticut", "delaware",
	"district of columbia", "florida", "georgia", "hawaii", "idaho", "illinois", "indiana", "iowa",
	"kansas", "kentucky", "louisiana", "maine", "maryland", "massachusetts", "michigan", "minnesota",
	"mississippi", "missouri", "montana", "nebraska", "nevada", "new hampshire", "new jersey",
	"new mexico", "new york", "north carolina", "north dakota", "ohio", "oklahoma", "oregon",
	"pennsylvania", "rhode island", "south carolina", "south dakota", "tennessee", "texas", "utah",
	"vermont", "virginia", "washington", "west virginia", "wisconsin", "wyoming",
}

var educationKeywords = []string{
	"associate degree", "bachelor", "b.a.", "b.s.", "certificate", "juris doctor", "j.d.", "ll.m.",
	"master", "paralegal certificate",
}

var certifications = []string{
	"certified paralegal", "notary public", "paralegal certificate", "shrm-cp",
}

var languages = []string{
	"arabic", "cantonese", "english", "french", "german", "hindi", "japanese", "korean", "mandarin",
	"portuguese", "russian", "spanish", "tagalog", "vietnamese",
}
