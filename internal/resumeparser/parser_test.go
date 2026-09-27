package resumeparser

import "testing"

func TestParsePlainTextResume(t *testing.T) {
	result, err := Parse("resume.txt", "text/plain", []byte(`Jane Doe
jane@example.com
(555) 123-4567
Paralegal with 6 years of legal experience in immigration and family law.
Skills include client intake, case management, e-filing, and legal research.
Spanish and English.
California Notary Public.
Bachelor of Arts and Paralegal Certificate.`))
	if err != nil {
		t.Fatalf("Parse returned error: %v", err)
	}
	if result.Email != "jane@example.com" {
		t.Fatalf("Email = %q", result.Email)
	}
	if result.YearsExperience == nil || *result.YearsExperience != 6 {
		t.Fatalf("YearsExperience = %v", result.YearsExperience)
	}
	if !contains(result.PracticeAreas, "immigration") || !contains(result.PracticeAreas, "family law") {
		t.Fatalf("PracticeAreas = %#v", result.PracticeAreas)
	}
	if !contains(result.Skills, "client intake") || !contains(result.Languages, "spanish") {
		t.Fatalf("Skills = %#v Languages = %#v", result.Skills, result.Languages)
	}
}

func TestParseLegacyDocUnsupported(t *testing.T) {
	_, err := Parse("resume.doc", "application/msword", []byte("binary"))
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestParseRTFResume(t *testing.T) {
	result, err := Parse("resume.rtf", "application/rtf", []byte(`{\rtf1\ansi Jane Doe\par jane@example.com\par 4 years of legal experience in personal injury.\par Spanish}`))
	if err != nil {
		t.Fatalf("Parse returned error: %v", err)
	}
	if result.Email != "jane@example.com" {
		t.Fatalf("Email = %q", result.Email)
	}
	if result.YearsExperience == nil || *result.YearsExperience != 4 {
		t.Fatalf("YearsExperience = %v", result.YearsExperience)
	}
	if !contains(result.PracticeAreas, "personal injury") {
		t.Fatalf("PracticeAreas = %#v", result.PracticeAreas)
	}
}

func contains(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}
