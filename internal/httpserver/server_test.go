package httpserver

import "testing"

func TestValidateCandidate(t *testing.T) {
	valid := CandidateRequest{
		Name:   "Avery Chen",
		Role:   "Contracts specialist",
		Focus:  "Commercial terms",
		Score:  94,
		Status: "Ready",
	}

	if err := validateCandidate(valid); err != nil {
		t.Fatalf("expected valid candidate, got error: %v", err)
	}
}

func TestValidateCandidateScoreRange(t *testing.T) {
	candidate := CandidateRequest{
		Name:   "Avery Chen",
		Role:   "Contracts specialist",
		Focus:  "Commercial terms",
		Score:  101,
		Status: "Ready",
	}

	if err := validateCandidate(candidate); err == nil {
		t.Fatal("expected validation error")
	}
}

func TestValidateInvitation(t *testing.T) {
	request := InviteUserRequest{
		Email: "avery@example.com",
		Name:  "Avery Chen",
		Role:  "platform_admin",
	}

	if err := validateInvitation(request); err != nil {
		t.Fatalf("expected valid invitation, got error: %v", err)
	}
}

func TestValidateInvitationRejectsInvalidRole(t *testing.T) {
	request := InviteUserRequest{
		Email: "avery@example.com",
		Name:  "Avery Chen",
		Role:  "owner",
	}

	if err := validateInvitation(request); err == nil {
		t.Fatal("expected validation error")
	}
}

func TestValidateJobApplication(t *testing.T) {
	application := JobApplicationRequest{
		FirstName: "Avery",
		LastName:  "Chen",
		Email:     "avery@example.com",
		Phone:     "555-123-4567",
	}

	if err := validateJobApplication(application); err != nil {
		t.Fatalf("expected valid job application, got error: %v", err)
	}
}

func TestValidateJobApplicationRejectsInvalidEmail(t *testing.T) {
	application := JobApplicationRequest{
		FirstName: "Avery",
		LastName:  "Chen",
		Email:     "avery.example.com",
		Phone:     "555-123-4567",
	}

	if err := validateJobApplication(application); err == nil {
		t.Fatal("expected validation error")
	}
}

func TestValidJobApplicationStatus(t *testing.T) {
	for _, status := range []string{"applied", "in_review", "rejected"} {
		if !validJobApplicationStatus(status) {
			t.Fatalf("expected %q to be valid", status)
		}
	}
}

func TestValidJobApplicationStatusRejectsConversionAndLegacyStatuses(t *testing.T) {
	for _, status := range []string{"hired", "in_progress", "declined"} {
		if validJobApplicationStatus(status) {
			t.Fatalf("expected %q to be invalid", status)
		}
	}
}

func TestResumePublicID(t *testing.T) {
	publicID := resumePublicID("/api/admin/resumes/abc123/download")
	if publicID != "abc123" {
		t.Fatalf("resumePublicID = %q", publicID)
	}
}

func TestResumePublicIDRejectsUnexpectedPath(t *testing.T) {
	publicID := resumePublicID("/api/admin/resumes/abc123")
	if publicID != "" {
		t.Fatalf("resumePublicID = %q", publicID)
	}
}

func TestValidResumeContentTypeAcceptsCommonResumeFormats(t *testing.T) {
	tests := []struct {
		name        string
		contentType string
		ext         string
	}{
		{name: "pdf", contentType: "application/pdf", ext: ".pdf"},
		{name: "doc", contentType: "application/msword", ext: ".doc"},
		{name: "docx", contentType: "application/vnd.openxmlformats-officedocument.wordprocessingml.document", ext: ".docx"},
		{name: "docx zip", contentType: "application/zip", ext: ".docx"},
		{name: "docx windows zip", contentType: "application/x-zip-compressed", ext: ".docx"},
		{name: "txt", contentType: "text/plain; charset=utf-8", ext: ".txt"},
		{name: "rtf", contentType: "application/rtf", ext: ".rtf"},
		{name: "rtf text", contentType: "text/rtf", ext: ".rtf"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if !validResumeContentType(test.contentType, test.ext) {
				t.Fatalf("expected %q %q to be valid", test.contentType, test.ext)
			}
		})
	}
}

func TestValidResumeFileSignatureAcceptsDocxZipSignature(t *testing.T) {
	data := []byte{'P', 'K', 0x03, 0x04, 'd', 'o', 'c', 'x'}
	if !validResumeFileSignature(data, ".docx") {
		t.Fatal("expected docx zip signature to be valid")
	}
}

func TestValidResumeContentTypeRejectsMismatchedFormat(t *testing.T) {
	if validResumeContentType("application/pdf", ".docx") {
		t.Fatal("expected mismatched content type to be invalid")
	}
}
