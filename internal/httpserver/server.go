package httpserver

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"legal-bench-api/internal/auth"
	"legal-bench-api/internal/db"
	"legal-bench-api/internal/objectstore"
	"legal-bench-api/internal/resumeparser"
)

const (
	sessionDuration    = 7 * 24 * time.Hour
	invitationDuration = 7 * 24 * time.Hour
	platformAdminRole  = "platform_admin"
	maxResumeBytes     = 5 << 20
)

type contextKey string

const currentUserKey contextKey = "currentUser"

type EmailSender interface {
	Configured() bool
	SendActivation(to string, name string, activationURL string) error
}

type Server struct {
	pool         *pgxpool.Pool
	queries      *db.Queries
	staticDir    string
	logger       *slog.Logger
	corsOrigin   map[string]struct{}
	frontendURL  string
	cookieSecure bool
	emailSender  EmailSender
	resumeStore  objectstore.Store
}

type CandidateRequest struct {
	Name   string `json:"name"`
	Role   string `json:"role"`
	Focus  string `json:"focus"`
	Score  int32  `json:"score"`
	Status string `json:"status"`
}

type JobApplicationRequest struct {
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
	Email     string `json:"email"`
	Phone     string `json:"phone"`
	ResumeURL string `json:"resume_url"`
}

type UpdateJobPostingStatusRequest struct {
	IsActive bool `json:"is_active"`
}

type UpdateJobApplicationStatusRequest struct {
	Status string `json:"status"`
}

type AuthRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type ActivateRequest struct {
	Token    string `json:"token"`
	Password string `json:"password"`
}

type InviteUserRequest struct {
	Email string `json:"email"`
	Name  string `json:"name"`
	Role  string `json:"role"`
}

type UpdateUserRoleRequest struct {
	Role string `json:"role"`
}

type currentUserResponse struct {
	ID     int64  `json:"id"`
	Email  string `json:"email"`
	Name   string `json:"name"`
	Role   string `json:"role"`
	Status string `json:"status"`
}

type candidateResponse struct {
	ID                     int64                `json:"id"`
	Name                   string               `json:"name"`
	Role                   string               `json:"role"`
	Focus                  string               `json:"focus"`
	Level                  string               `json:"level"`
	Score                  int32                `json:"score"`
	Status                 string               `json:"status"`
	CreatedAt              time.Time            `json:"created_at"`
	UpdatedAt              time.Time            `json:"updated_at"`
	FirstName              string               `json:"first_name"`
	LastName               string               `json:"last_name"`
	Email                  string               `json:"email"`
	Phone                  string               `json:"phone"`
	Experience             string               `json:"experience"`
	CoverLetter            string               `json:"cover_letter"`
	ResumeURL              string               `json:"resume_url"`
	SourceJobPostingID     *int64               `json:"source_job_posting_id"`
	SourceJobApplicationID *int64               `json:"source_job_application_id"`
	SourceJobTitle         string               `json:"source_job_title"`
	AvailabilityStatus     string               `json:"availability_status"`
	ContractStatus         string               `json:"contract_status"`
	HiredAt                *time.Time           `json:"hired_at"`
	ResumeParse            *resumeParseResponse `json:"resume_parse,omitempty"`
}

type resumeParseResponse struct {
	Status          string     `json:"status"`
	Summary         string     `json:"summary"`
	YearsExperience *int32     `json:"years_experience"`
	Skills          []string   `json:"skills"`
	PracticeAreas   []string   `json:"practice_areas"`
	Jurisdictions   []string   `json:"jurisdictions"`
	Education       []string   `json:"education"`
	Certifications  []string   `json:"certifications"`
	Languages       []string   `json:"languages"`
	Error           string     `json:"error"`
	ParsedAt        *time.Time `json:"parsed_at"`
}

type jobApplicationsResponse struct {
	Items    []db.ListJobApplicationsRow `json:"items"`
	Total    int64                       `json:"total"`
	Page     int32                       `json:"page"`
	PageSize int32                       `json:"page_size"`
	Status   string                      `json:"status"`
}

type invitationResponse struct {
	ID            int64     `json:"id"`
	Email         string    `json:"email"`
	Name          string    `json:"name"`
	Role          string    `json:"role"`
	ExpiresAt     time.Time `json:"expires_at"`
	ActivationURL string    `json:"activation_url"`
}

func New(pool *pgxpool.Pool, staticDir string, corsOrigins string, frontendURL string, cookieSecure bool, emailSender EmailSender, resumeStore objectstore.Store, logger *slog.Logger) http.Handler {
	if logger == nil {
		logger = slog.Default()
	}

	server := &Server{
		pool:         pool,
		queries:      db.New(pool),
		staticDir:    staticDir,
		logger:       logger,
		corsOrigin:   parseAllowedOrigins(corsOrigins),
		frontendURL:  strings.TrimRight(frontendURL, "/"),
		cookieSecure: cookieSecure,
		emailSender:  emailSender,
		resumeStore:  resumeStore,
	}

	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/auth/login", server.login)
	mux.HandleFunc("POST /api/auth/logout", server.logout)
	mux.HandleFunc("POST /api/auth/activate", server.activate)
	mux.HandleFunc("GET /api/auth/me", server.requireAuth(server.me))
	mux.HandleFunc("GET /api/health", server.health)
	mux.HandleFunc("GET /api/candidates", server.requireAuth(server.listCandidates))
	mux.HandleFunc("POST /api/candidates", server.requireAuth(server.createCandidate))
	mux.HandleFunc("GET /api/jobs/{slug}", server.getJobPosting)
	mux.HandleFunc("POST /api/jobs/{slug}/applications", server.createJobApplication)
	mux.HandleFunc("GET /api/admin/jobs", server.requirePlatformAdmin(server.listJobPostings))
	mux.HandleFunc("PATCH /api/admin/jobs/{id}/status", server.requirePlatformAdmin(server.updateJobPostingStatus))
	mux.HandleFunc("GET /api/admin/applicants", server.requirePlatformAdmin(server.listJobApplications))
	mux.HandleFunc("PATCH /api/admin/applicants/{id}/status", server.requirePlatformAdmin(server.updateJobApplicationStatus))
	mux.HandleFunc("POST /api/admin/applicants/{id}/hire", server.requirePlatformAdmin(server.hireJobApplicant))
	mux.HandleFunc("GET /api/admin/resumes/{publicID}/download", server.requirePlatformAdmin(server.downloadResume))
	mux.HandleFunc("GET /api/admin/users", server.requirePlatformAdmin(server.listUsers))
	mux.HandleFunc("POST /api/admin/users/invitations", server.requirePlatformAdmin(server.inviteUser))
	mux.HandleFunc("PATCH /api/admin/users/{id}/role", server.requirePlatformAdmin(server.updateUserRole))
	mux.HandleFunc("PATCH /api/admin/users/{id}/status", server.requirePlatformAdmin(server.updateUserStatus))
	mux.HandleFunc("/api/", server.notFound)
	mux.Handle("/", spaFileServer(staticDir))

	return server.logRequests(server.securityHeaders(server.cors(mux)))
}

func (s *Server) securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
		w.Header().Set("Permissions-Policy", "camera=(), microphone=(), geolocation=()")
		if s.cookieSecure {
			w.Header().Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
		}
		next.ServeHTTP(w, r)
	})
}

func (s *Server) health(w http.ResponseWriter, r *http.Request) {
	dbStatus := "ok"
	if err := s.pool.Ping(r.Context()); err != nil {
		dbStatus = "error"
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok", "database": dbStatus})
}

func (s *Server) login(w http.ResponseWriter, r *http.Request) {
	var request AuthRequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}

	request.Email = strings.TrimSpace(strings.ToLower(request.Email))
	if request.Email == "" || request.Password == "" {
		writeError(w, http.StatusBadRequest, "email and password are required")
		return
	}

	user, err := s.queries.GetUserByEmail(r.Context(), request.Email)
	if err != nil || !auth.CheckPassword(request.Password, user.PasswordHash) || user.Status != "active" {
		writeError(w, http.StatusUnauthorized, "invalid email or password")
		return
	}

	s.writeSession(w, r, user)
}

func (s *Server) activate(w http.ResponseWriter, r *http.Request) {
	var request ActivateRequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	if request.Token == "" || len(request.Password) < 12 {
		writeError(w, http.StatusBadRequest, "token and a password of at least 12 characters are required")
		return
	}

	invitation, err := s.queries.GetUserInvitationByTokenHash(r.Context(), auth.HashSessionToken(request.Token))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invitation is invalid or expired")
		return
	}

	passwordHash, err := auth.HashPassword(request.Password)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not activate account")
		return
	}

	user, err := s.queries.CreateUser(r.Context(), db.CreateUserParams{
		Email:        invitation.Email,
		PasswordHash: passwordHash,
		Name:         invitation.Name,
		Role:         invitation.Role,
	})
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			writeError(w, http.StatusConflict, "user already exists")
			return
		}
		writeError(w, http.StatusInternalServerError, "could not activate account")
		return
	}

	_, _ = s.queries.AcceptUserInvitation(r.Context(), db.AcceptUserInvitationParams{ID: invitation.ID, AcceptedBy: user.ID})
	s.writeSession(w, r, user)
}

func (s *Server) logout(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie(auth.SessionCookieName)
	if err == nil && cookie.Value != "" {
		_ = s.queries.DeleteSessionByTokenHash(r.Context(), auth.HashSessionToken(cookie.Value))
	}
	http.SetCookie(w, auth.ExpiredSessionCookie(s.cookieSecure))
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) me(w http.ResponseWriter, r *http.Request) {
	user, ok := currentUserFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "authentication required")
		return
	}
	writeJSON(w, http.StatusOK, toCurrentUserResponse(user))
}

func (s *Server) listUsers(w http.ResponseWriter, r *http.Request) {
	users, err := s.queries.ListUsers(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not load users")
		return
	}
	responses := make([]currentUserResponse, 0, len(users))
	for _, user := range users {
		responses = append(responses, toCurrentUserResponse(user))
	}
	writeJSON(w, http.StatusOK, responses)
}

func (s *Server) inviteUser(w http.ResponseWriter, r *http.Request) {
	currentUser, _ := currentUserFromContext(r.Context())
	var request InviteUserRequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	request.Email = strings.TrimSpace(strings.ToLower(request.Email))
	request.Name = strings.TrimSpace(request.Name)
	request.Role = strings.TrimSpace(request.Role)
	if err := validateInvitation(request); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if _, err := s.queries.GetUserByEmail(r.Context(), request.Email); err == nil {
		writeError(w, http.StatusConflict, "user already exists")
		return
	} else if !errors.Is(err, pgx.ErrNoRows) {
		writeError(w, http.StatusInternalServerError, "could not check user")
		return
	}

	token, err := auth.NewSessionToken()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not create invitation")
		return
	}
	expiresAt := time.Now().Add(invitationDuration)
	invitation, err := s.queries.CreateUserInvitation(r.Context(), db.CreateUserInvitationParams{
		Email:     request.Email,
		Name:      request.Name,
		Role:      request.Role,
		TokenHash: auth.HashSessionToken(token),
		InvitedBy: currentUser.ID,
		ExpiresAt: expiresAt,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not create invitation")
		return
	}

	activationURL := s.activationURL(token)
	if s.emailSender != nil && s.emailSender.Configured() {
		if err := s.emailSender.SendActivation(invitation.Email, invitation.Name, activationURL); err != nil {
			s.logger.Error("send activation email", "email", invitation.Email, "error", err)
			writeError(w, http.StatusInternalServerError, "invitation created but email could not be sent")
			return
		}
		s.logger.Info("user invitation email sent", "email", invitation.Email)
	} else {
		s.logger.Info("user invitation created", "email", invitation.Email, "activation_url", activationURL)
	}
	writeJSON(w, http.StatusCreated, invitationResponse{ID: invitation.ID, Email: invitation.Email, Name: invitation.Name, Role: invitation.Role, ExpiresAt: invitation.ExpiresAt, ActivationURL: activationURL})
}

func (s *Server) updateUserRole(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid user id")
		return
	}
	var request UpdateUserRoleRequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	request.Role = strings.TrimSpace(request.Role)
	if !validRole(request.Role) {
		writeError(w, http.StatusBadRequest, "role is invalid")
		return
	}
	user, err := s.queries.UpdateUserRole(r.Context(), db.UpdateUserRoleParams{ID: id, Role: request.Role})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not update user role")
		return
	}
	writeJSON(w, http.StatusOK, toCurrentUserResponse(user))
}

func (s *Server) updateUserStatus(w http.ResponseWriter, r *http.Request) {
	currentUser, _ := currentUserFromContext(r.Context())
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid user id")
		return
	}
	if id == currentUser.ID {
		writeError(w, http.StatusBadRequest, "you cannot disable your own account")
		return
	}
	user, err := s.queries.UpdateUserStatus(r.Context(), db.UpdateUserStatusParams{ID: id, Status: "disabled"})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not disable user")
		return
	}
	writeJSON(w, http.StatusOK, toCurrentUserResponse(user))
}

func (s *Server) listCandidates(w http.ResponseWriter, r *http.Request) {
	candidates, err := s.queries.ListCandidates(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not load candidates")
		return
	}
	responses := make([]candidateResponse, 0, len(candidates))
	for _, candidate := range candidates {
		responses = append(responses, toCandidateListResponse(candidate))
	}
	writeJSON(w, http.StatusOK, responses)
}

func (s *Server) createCandidate(w http.ResponseWriter, r *http.Request) {
	var request CandidateRequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	if err := validateCandidate(request); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	candidate, err := s.queries.CreateCandidate(r.Context(), db.CreateCandidateParams{Name: strings.TrimSpace(request.Name), Role: strings.TrimSpace(request.Role), Focus: strings.TrimSpace(request.Focus), Score: request.Score, Status: strings.TrimSpace(request.Status)})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not create candidate")
		return
	}
	writeJSON(w, http.StatusCreated, toCandidateResponse(candidate))
}

func (s *Server) listJobPostings(w http.ResponseWriter, r *http.Request) {
	jobs, err := s.queries.ListJobPostings(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not load job postings")
		return
	}
	writeJSON(w, http.StatusOK, jobs)
}

func (s *Server) getJobPosting(w http.ResponseWriter, r *http.Request) {
	job, err := s.queries.GetJobPostingBySlug(r.Context(), strings.TrimSpace(r.PathValue("slug")))
	if errors.Is(err, pgx.ErrNoRows) {
		writeError(w, http.StatusNotFound, "job posting not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not load job posting")
		return
	}
	writeJSON(w, http.StatusOK, job)
}

func (s *Server) updateJobPostingStatus(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid job posting id")
		return
	}
	var request UpdateJobPostingStatusRequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	job, err := s.queries.UpdateJobPostingStatus(r.Context(), db.UpdateJobPostingStatusParams{ID: id, IsActive: request.IsActive})
	if errors.Is(err, pgx.ErrNoRows) {
		writeError(w, http.StatusNotFound, "job posting not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not update job posting")
		return
	}
	writeJSON(w, http.StatusOK, job)
}

func (s *Server) listJobApplications(w http.ResponseWriter, r *http.Request) {
	status := strings.TrimSpace(r.URL.Query().Get("status"))
	if status != "" && !validJobApplicationStatus(status) {
		writeError(w, http.StatusBadRequest, "applicant status is invalid")
		return
	}

	page := parseQueryInt32(r, "page", 1, 1, 100000)
	pageSize := parseQueryInt32(r, "page_size", 25, 1, 100)
	offset := (page - 1) * pageSize

	applications, err := s.queries.ListJobApplications(r.Context(), db.ListJobApplicationsParams{
		Status: status,
		Limit:  pageSize,
		Offset: offset,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not load applicants")
		return
	}
	total, err := s.queries.CountJobApplications(r.Context(), status)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not count applicants")
		return
	}

	writeJSON(w, http.StatusOK, jobApplicationsResponse{Items: applications, Total: total, Page: page, PageSize: pageSize, Status: status})
}

func (s *Server) updateJobApplicationStatus(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid applicant id")
		return
	}
	var request UpdateJobApplicationStatusRequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	request.Status = strings.TrimSpace(request.Status)
	if !validJobApplicationStatus(request.Status) {
		writeError(w, http.StatusBadRequest, "applicant status is invalid")
		return
	}

	application, err := s.queries.UpdateJobApplicationStatus(r.Context(), db.UpdateJobApplicationStatusParams{ID: id, Status: request.Status})
	if errors.Is(err, pgx.ErrNoRows) {
		writeError(w, http.StatusNotFound, "applicant not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not update applicant")
		return
	}
	writeJSON(w, http.StatusOK, application)
}

func (s *Server) hireJobApplicant(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid applicant id")
		return
	}

	tx, err := s.pool.Begin(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not hire applicant")
		return
	}
	defer tx.Rollback(r.Context())

	queries := s.queries.WithTx(tx)
	application, err := queries.GetJobApplicationForHire(r.Context(), id)
	if errors.Is(err, pgx.ErrNoRows) {
		writeError(w, http.StatusNotFound, "applicant not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not load applicant")
		return
	}

	candidate, err := queries.CreateCandidateFromApplication(r.Context(), db.CreateCandidateFromApplicationParams{
		Name:                   strings.TrimSpace(application.FirstName + " " + application.LastName),
		Role:                   application.JobTitle,
		Focus:                  application.Experience,
		FirstName:              application.FirstName,
		LastName:               application.LastName,
		Email:                  application.Email,
		Phone:                  application.Phone,
		Experience:             application.Experience,
		CoverLetter:            application.CoverLetter,
		ResumeUrl:              application.ResumeUrl,
		SourceJobPostingID:     pgtype.Int8{Int64: application.JobPostingID, Valid: true},
		SourceJobApplicationID: pgtype.Int8{Int64: application.ID, Valid: true},
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not create candidate")
		return
	}
	if err := queries.DeleteJobApplication(r.Context(), application.ID); err != nil {
		writeError(w, http.StatusInternalServerError, "could not remove applicant")
		return
	}
	if err := tx.Commit(r.Context()); err != nil {
		writeError(w, http.StatusInternalServerError, "could not hire applicant")
		return
	}

	s.enqueueResumeParse(candidate.ID, candidate.ResumeUrl)
	writeJSON(w, http.StatusCreated, toCandidateResponse(candidate))
}

func (s *Server) createJobApplication(w http.ResponseWriter, r *http.Request) {
	job, err := s.queries.GetJobPostingBySlug(r.Context(), strings.TrimSpace(r.PathValue("slug")))
	if errors.Is(err, pgx.ErrNoRows) {
		writeError(w, http.StatusNotFound, "job posting not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not load job posting")
		return
	}
	if !job.IsActive {
		writeError(w, http.StatusGone, "this job is no longer available")
		return
	}

	var request JobApplicationRequest
	if strings.HasPrefix(r.Header.Get("Content-Type"), "multipart/form-data") {
		r.Body = http.MaxBytesReader(w, r.Body, maxResumeBytes+1<<20)
		if err := r.ParseMultipartForm(maxResumeBytes); err != nil {
			writeError(w, http.StatusBadRequest, "invalid application form")
			return
		}

		request = JobApplicationRequest{
			FirstName: r.FormValue("first_name"),
			LastName:  r.FormValue("last_name"),
			Email:     r.FormValue("email"),
			Phone:     r.FormValue("phone"),
		}

		resumeURL, err := s.saveResumeUpload(r)
		if errors.Is(err, errInvalidResume) {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		if err != nil {
			s.logger.Error("save resume attachment", "error", err)
			writeError(w, http.StatusInternalServerError, "could not save resume attachment")
			return
		}
		request.ResumeURL = resumeURL
	} else {
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			writeError(w, http.StatusBadRequest, "invalid JSON body")
			return
		}
	}
	normalizeJobApplication(&request)
	if err := validateJobApplication(request); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	application, err := s.queries.CreateJobApplication(r.Context(), db.CreateJobApplicationParams{
		JobPostingID: job.ID,
		FirstName:    request.FirstName,
		LastName:     request.LastName,
		Email:        request.Email,
		Phone:        request.Phone,
		Experience:   "",
		CoverLetter:  "",
		ResumeUrl:    request.ResumeURL,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not submit application")
		return
	}
	writeJSON(w, http.StatusCreated, application)
}

var errInvalidResume = errors.New("invalid resume attachment")

func (s *Server) saveResumeUpload(r *http.Request) (string, error) {
	file, header, err := r.FormFile("resume")
	if errors.Is(err, http.ErrMissingFile) {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("%w: could not read resume attachment", errInvalidResume)
	}
	defer file.Close()

	if header.Size > maxResumeBytes {
		return "", fmt.Errorf("%w: resume attachment must be 5 MB or smaller", errInvalidResume)
	}

	filename := sanitizeResumeFilename(header.Filename)
	ext := strings.ToLower(filepath.Ext(filename))
	switch ext {
	case ".pdf", ".doc", ".docx", ".txt", ".rtf":
	default:
		return "", fmt.Errorf("%w: resume attachment must be a PDF, DOC, DOCX, TXT, or RTF file", errInvalidResume)
	}

	data, err := io.ReadAll(io.LimitReader(file, maxResumeBytes+1))
	if err != nil {
		return "", err
	}
	if len(data) > maxResumeBytes {
		return "", fmt.Errorf("%w: resume attachment must be 5 MB or smaller", errInvalidResume)
	}

	contentType := strings.TrimSpace(header.Header.Get("Content-Type"))
	detectedContentType := http.DetectContentType(data)
	if contentType == "" || strings.EqualFold(strings.Split(contentType, ";")[0], "application/octet-stream") {
		contentType = detectedContentType
	}
	if !validResumeContentType(contentType, ext) && !validResumeContentType(detectedContentType, ext) && !validResumeFileSignature(data, ext) {
		return "", fmt.Errorf("%w: resume attachment content type is not allowed", errInvalidResume)
	}

	publicID, err := randomHex(16)
	if err != nil {
		return "", err
	}
	storageKey := fmt.Sprintf("resumes/%s/%s/%s", time.Now().UTC().Format("2006/01"), publicID, filename)
	if err := s.resumeStore.Put(r.Context(), storageKey, contentType, data); err != nil {
		return "", fmt.Errorf("store resume object: %w", err)
	}

	resumeFile, err := s.queries.CreateResumeFile(r.Context(), db.CreateResumeFileParams{
		PublicID:         publicID,
		StorageKey:       storageKey,
		OriginalFilename: filename,
		ContentType:      contentType,
		SizeBytes:        int64(len(data)),
	})
	if err != nil {
		return "", fmt.Errorf("create resume file record: %w", err)
	}

	return "/api/admin/resumes/" + url.PathEscape(resumeFile.PublicID) + "/download", nil
}

func (s *Server) downloadResume(w http.ResponseWriter, r *http.Request) {
	resumeFile, err := s.queries.GetResumeFileByPublicID(r.Context(), strings.TrimSpace(r.PathValue("publicID")))
	if errors.Is(err, pgx.ErrNoRows) {
		writeError(w, http.StatusNotFound, "resume not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not load resume metadata")
		return
	}

	object, err := s.resumeStore.Get(r.Context(), resumeFile.StorageKey)
	if err != nil {
		s.logger.Error("read resume object", "resume_id", resumeFile.PublicID, "error", err)
		writeError(w, http.StatusInternalServerError, "could not load resume")
		return
	}
	defer object.Body.Close()

	contentType := resumeFile.ContentType
	if contentType == "" {
		contentType = object.ContentType
	}
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Content-Disposition", "attachment; filename=\""+downloadFilename(resumeFile.OriginalFilename)+"\"")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	if resumeFile.SizeBytes > 0 {
		w.Header().Set("Content-Length", strconv.FormatInt(resumeFile.SizeBytes, 10))
	}
	_, _ = io.Copy(w, object.Body)
}

func (s *Server) enqueueResumeParse(candidateID int64, resumeURL string) {
	publicID := resumePublicID(resumeURL)
	if publicID == "" {
		s.logger.Info("candidate has no resume to parse", "candidate_id", candidateID)
		return
	}

	go s.parseResumeForCandidate(candidateID, publicID)
}

func (s *Server) parseResumeForCandidate(candidateID int64, publicID string) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	resumeFile, err := s.queries.GetResumeFileByPublicID(ctx, publicID)
	if err != nil {
		s.logger.Error("load resume metadata for parsing", "candidate_id", candidateID, "resume_id", publicID, "error", err)
		return
	}

	if _, err := s.queries.StartCandidateResumeParse(ctx, db.StartCandidateResumeParseParams{
		CandidateID:   candidateID,
		ResumeFileID:  resumeFile.ID,
		ParserVersion: resumeparser.Version,
	}); err != nil {
		s.logger.Error("start resume parse", "candidate_id", candidateID, "resume_id", publicID, "error", err)
		return
	}

	object, err := s.resumeStore.Get(ctx, resumeFile.StorageKey)
	if err != nil {
		s.failResumeParse(ctx, candidateID, err)
		s.logger.Error("read resume object for parsing", "candidate_id", candidateID, "resume_id", publicID, "error", err)
		return
	}
	defer object.Body.Close()

	data, err := io.ReadAll(io.LimitReader(object.Body, maxResumeBytes+1))
	if err != nil {
		s.failResumeParse(ctx, candidateID, err)
		s.logger.Error("read resume body for parsing", "candidate_id", candidateID, "resume_id", publicID, "error", err)
		return
	}
	if len(data) > maxResumeBytes {
		err := fmt.Errorf("resume exceeds parser size limit")
		s.failResumeParse(ctx, candidateID, err)
		s.logger.Error("resume too large for parsing", "candidate_id", candidateID, "resume_id", publicID)
		return
	}

	result, err := resumeparser.Parse(resumeFile.OriginalFilename, resumeFile.ContentType, data)
	if err != nil {
		s.failResumeParse(ctx, candidateID, err)
		s.logger.Error("parse resume", "candidate_id", candidateID, "resume_id", publicID, "error", err)
		return
	}

	params := db.CompleteCandidateResumeParseParams{
		CandidateID:    candidateID,
		ParserVersion:  resumeparser.Version,
		ParsedText:     result.Text,
		Summary:        result.Summary,
		Email:          result.Email,
		Phone:          result.Phone,
		Skills:         result.Skills,
		PracticeAreas:  result.PracticeAreas,
		Jurisdictions:  result.Jurisdictions,
		Education:      result.Education,
		Certifications: result.Certifications,
		Languages:      result.Languages,
	}
	if result.YearsExperience != nil {
		params.YearsExperience = pgtype.Int4{Int32: *result.YearsExperience, Valid: true}
	}
	if _, err := s.queries.CompleteCandidateResumeParse(ctx, params); err != nil {
		s.logger.Error("complete resume parse", "candidate_id", candidateID, "resume_id", publicID, "error", err)
		return
	}
	s.logger.Info("resume parsed", "candidate_id", candidateID, "resume_id", publicID)
}

func (s *Server) failResumeParse(ctx context.Context, candidateID int64, cause error) {
	message := strings.TrimSpace(cause.Error())
	if len(message) > 1000 {
		message = message[:1000]
	}
	if _, err := s.queries.FailCandidateResumeParse(ctx, db.FailCandidateResumeParseParams{
		CandidateID:  candidateID,
		ErrorMessage: message,
	}); err != nil {
		s.logger.Error("mark resume parse failed", "candidate_id", candidateID, "error", err)
	}
}

func resumePublicID(resumeURL string) string {
	resumeURL = strings.TrimSpace(resumeURL)
	if resumeURL == "" {
		return ""
	}
	parsed, err := url.Parse(resumeURL)
	if err != nil {
		return ""
	}
	if strings.Trim(parsed.Path, "/") == "" {
		return ""
	}
	resumeDir := filepath.Dir(parsed.Path)
	if filepath.Base(filepath.Dir(resumeDir)) != "resumes" || filepath.Base(parsed.Path) != "download" {
		return ""
	}
	return filepath.Base(resumeDir)
}

func validResumeContentType(contentType string, ext string) bool {
	contentType = strings.ToLower(strings.TrimSpace(strings.Split(contentType, ";")[0]))
	switch ext {
	case ".pdf":
		return contentType == "application/pdf"
	case ".doc":
		return contentType == "application/msword" || contentType == "application/octet-stream"
	case ".docx":
		return contentType == "application/vnd.openxmlformats-officedocument.wordprocessingml.document" || contentType == "application/zip" || contentType == "application/x-zip-compressed" || contentType == "application/octet-stream"
	case ".txt":
		return contentType == "text/plain" || contentType == "application/octet-stream"
	case ".rtf":
		return contentType == "application/rtf" || contentType == "application/x-rtf" || contentType == "text/rtf" || contentType == "text/plain" || contentType == "application/octet-stream"
	default:
		return false
	}
}

func validResumeFileSignature(data []byte, ext string) bool {
	switch ext {
	case ".docx":
		return len(data) >= 4 && data[0] == 'P' && data[1] == 'K' && (data[2] == 0x03 || data[2] == 0x05 || data[2] == 0x07) && (data[3] == 0x04 || data[3] == 0x06 || data[3] == 0x08)
	case ".pdf":
		return len(data) >= 5 && string(data[:5]) == "%PDF-"
	case ".rtf":
		return len(data) >= 5 && strings.HasPrefix(string(data[:5]), "{\\rtf")
	default:
		return false
	}
}

func downloadFilename(filename string) string {
	filename = sanitizeResumeFilename(filename)
	return strings.ReplaceAll(filename, "\"", "-")
}

func randomHex(byteCount int) (string, error) {
	bytes := make([]byte, byteCount)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes), nil
}

func sanitizeResumeFilename(filename string) string {
	base := strings.TrimSpace(filepath.Base(filename))
	if base == "" || base == "." {
		return "resume"
	}

	var builder strings.Builder
	for _, char := range base {
		switch {
		case char >= 'a' && char <= 'z':
			builder.WriteRune(char)
		case char >= 'A' && char <= 'Z':
			builder.WriteRune(char)
		case char >= '0' && char <= '9':
			builder.WriteRune(char)
		case char == '.', char == '-', char == '_':
			builder.WriteRune(char)
		default:
			builder.WriteRune('-')
		}
	}

	cleaned := strings.Trim(builder.String(), ".-")
	if cleaned == "" {
		return "resume"
	}
	return cleaned
}

func (s *Server) writeSession(w http.ResponseWriter, r *http.Request, user db.User) {
	token, err := auth.NewSessionToken()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not create session")
		return
	}
	expiresAt := time.Now().Add(sessionDuration)
	_, err = s.queries.CreateSession(r.Context(), db.CreateSessionParams{UserID: user.ID, TokenHash: auth.HashSessionToken(token), ExpiresAt: expiresAt})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not create session")
		return
	}
	http.SetCookie(w, auth.SessionCookie(token, expiresAt, s.cookieSecure))
	writeJSON(w, http.StatusOK, toCurrentUserResponse(user))
}

func (s *Server) requireAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user, ok := s.authenticateRequest(w, r)
		if !ok {
			return
		}
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), currentUserKey, user)))
	}
}

func (s *Server) requirePlatformAdmin(next http.HandlerFunc) http.HandlerFunc {
	return s.requireAuth(func(w http.ResponseWriter, r *http.Request) {
		user, _ := currentUserFromContext(r.Context())
		if user.Role != platformAdminRole {
			writeError(w, http.StatusForbidden, "platform admin access required")
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (s *Server) authenticateRequest(w http.ResponseWriter, r *http.Request) (db.User, bool) {
	cookie, err := r.Cookie(auth.SessionCookieName)
	if err != nil || cookie.Value == "" {
		writeError(w, http.StatusUnauthorized, "authentication required")
		return db.User{}, false
	}
	session, err := s.queries.GetSessionByTokenHash(r.Context(), auth.HashSessionToken(cookie.Value))
	if err != nil {
		writeError(w, http.StatusUnauthorized, "authentication required")
		return db.User{}, false
	}
	user, err := s.queries.GetUserByID(r.Context(), session.UserID)
	if err != nil || user.Status != "active" {
		writeError(w, http.StatusUnauthorized, "authentication required")
		return db.User{}, false
	}
	return user, true
}

func (s *Server) activationURL(token string) string {
	frontendURL := s.frontendURL
	if frontendURL == "" {
		frontendURL = "http://localhost:5173"
	}
	return fmt.Sprintf("%s/activate?token=%s", frontendURL, url.QueryEscape(token))
}

func (s *Server) notFound(w http.ResponseWriter, r *http.Request) {
	writeError(w, http.StatusNotFound, "not found")
}

func (s *Server) cors(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		if _, ok := s.corsOrigin[origin]; ok {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Vary", "Origin")
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PATCH, DELETE, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
			w.Header().Set("Access-Control-Allow-Credentials", "true")
		}
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (s *Server) logRequests(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		next.ServeHTTP(w, r)
		s.logger.Info("request", "method", r.Method, "path", r.URL.Path)
	})
}

func currentUserFromContext(ctx context.Context) (db.User, bool) {
	user, ok := ctx.Value(currentUserKey).(db.User)
	return user, ok
}

func parseAllowedOrigins(origins string) map[string]struct{} {
	allowed := map[string]struct{}{}
	for _, origin := range strings.Split(origins, ",") {
		origin = strings.TrimSpace(origin)
		if origin != "" {
			allowed[origin] = struct{}{}
		}
	}
	return allowed
}

func toCurrentUserResponse(user db.User) currentUserResponse {
	return currentUserResponse{ID: user.ID, Email: user.Email, Name: user.Name, Role: user.Role, Status: user.Status}
}

func toCandidateResponse(candidate db.Candidate) candidateResponse {
	var sourceJobPostingID *int64
	if candidate.SourceJobPostingID.Valid {
		sourceJobPostingID = &candidate.SourceJobPostingID.Int64
	}
	var sourceJobApplicationID *int64
	if candidate.SourceJobApplicationID.Valid {
		sourceJobApplicationID = &candidate.SourceJobApplicationID.Int64
	}
	var hiredAt *time.Time
	if candidate.HiredAt.Valid {
		hiredAt = &candidate.HiredAt.Time
	}

	return candidateResponse{
		ID:                     candidate.ID,
		Name:                   candidate.Name,
		Role:                   candidate.Role,
		Focus:                  candidate.Focus,
		Level:                  inferCandidateLevel(candidate.Focus, nil),
		Score:                  candidate.Score,
		Status:                 candidate.Status,
		CreatedAt:              candidate.CreatedAt,
		UpdatedAt:              candidate.UpdatedAt,
		FirstName:              candidate.FirstName,
		LastName:               candidate.LastName,
		Email:                  candidate.Email,
		Phone:                  candidate.Phone,
		Experience:             candidate.Experience,
		CoverLetter:            candidate.CoverLetter,
		ResumeURL:              candidate.ResumeUrl,
		SourceJobPostingID:     sourceJobPostingID,
		SourceJobApplicationID: sourceJobApplicationID,
		AvailabilityStatus:     candidate.AvailabilityStatus,
		ContractStatus:         candidate.ContractStatus,
		HiredAt:                hiredAt,
	}
}

func toCandidateListResponse(candidate db.ListCandidatesRow) candidateResponse {
	var sourceJobPostingID *int64
	if candidate.SourceJobPostingID.Valid {
		sourceJobPostingID = &candidate.SourceJobPostingID.Int64
	}
	var sourceJobApplicationID *int64
	if candidate.SourceJobApplicationID.Valid {
		sourceJobApplicationID = &candidate.SourceJobApplicationID.Int64
	}
	var hiredAt *time.Time
	if candidate.HiredAt.Valid {
		hiredAt = &candidate.HiredAt.Time
	}

	var yearsExperience *int32
	if candidate.YearsExperience.Valid {
		yearsExperience = &candidate.YearsExperience.Int32
	}

	var parsedAt *time.Time
	if candidate.ParsedAt.Valid {
		parsedAt = &candidate.ParsedAt.Time
	}

	var resumeParse *resumeParseResponse
	if candidate.ResumeParseStatus != "" {
		resumeParse = &resumeParseResponse{
			Status:          candidate.ResumeParseStatus,
			Summary:         candidate.ResumeSummary,
			YearsExperience: yearsExperience,
			Skills:          candidate.Skills,
			PracticeAreas:   candidate.PracticeAreas,
			Jurisdictions:   candidate.Jurisdictions,
			Education:       candidate.Education,
			Certifications:  candidate.Certifications,
			Languages:       candidate.Languages,
			Error:           candidate.ResumeParseError,
			ParsedAt:        parsedAt,
		}
	}

	return candidateResponse{
		ID:                     candidate.ID,
		Name:                   candidate.Name,
		Role:                   candidate.Role,
		Focus:                  candidate.Focus,
		Level:                  inferCandidateLevel(candidate.Experience+" "+candidate.Focus+" "+candidate.ResumeSummary, yearsExperience),
		Score:                  candidate.Score,
		Status:                 candidate.Status,
		CreatedAt:              candidate.CreatedAt,
		UpdatedAt:              candidate.UpdatedAt,
		FirstName:              candidate.FirstName,
		LastName:               candidate.LastName,
		Email:                  candidate.Email,
		Phone:                  candidate.Phone,
		Experience:             candidate.Experience,
		CoverLetter:            candidate.CoverLetter,
		ResumeURL:              candidate.ResumeUrl,
		SourceJobPostingID:     sourceJobPostingID,
		SourceJobApplicationID: sourceJobApplicationID,
		SourceJobTitle:         candidate.SourceJobTitle,
		AvailabilityStatus:     candidate.AvailabilityStatus,
		ContractStatus:         candidate.ContractStatus,
		HiredAt:                hiredAt,
		ResumeParse:            resumeParse,
	}
}

func inferCandidateLevel(text string, yearsExperience *int32) string {
	if yearsExperience != nil {
		switch {
		case *yearsExperience >= 7:
			return "Senior"
		case *yearsExperience >= 3:
			return "Mid"
		default:
			return "Entry"
		}
	}

	text = strings.ToLower(text)
	switch {
	case strings.Contains(text, "senior") || strings.Contains(text, "lead") || strings.Contains(text, "supervisor"):
		return "Senior"
	case strings.Contains(text, "junior") || strings.Contains(text, "entry"):
		return "Entry"
	default:
		return "Not rated"
	}
}

func validateInvitation(request InviteUserRequest) error {
	switch {
	case request.Email == "":
		return errors.New("email is required")
	case !strings.Contains(request.Email, "@"):
		return errors.New("email must be valid")
	case request.Name == "":
		return errors.New("name is required")
	case !validRole(request.Role):
		return errors.New("role is invalid")
	default:
		return nil
	}
}

func validRole(role string) bool {
	switch role {
	case "platform_admin", "admin", "law_firm_user", "contractor", "reviewer":
		return true
	default:
		return false
	}
}

func validateCandidate(candidate CandidateRequest) error {
	switch {
	case strings.TrimSpace(candidate.Name) == "":
		return errors.New("name is required")
	case strings.TrimSpace(candidate.Role) == "":
		return errors.New("role is required")
	case strings.TrimSpace(candidate.Focus) == "":
		return errors.New("focus is required")
	case strings.TrimSpace(candidate.Status) == "":
		return errors.New("status is required")
	case candidate.Score < 0 || candidate.Score > 100:
		return errors.New("score must be between 0 and 100")
	default:
		return nil
	}
}

func normalizeJobApplication(application *JobApplicationRequest) {
	application.FirstName = strings.TrimSpace(application.FirstName)
	application.LastName = strings.TrimSpace(application.LastName)
	application.Email = strings.TrimSpace(strings.ToLower(application.Email))
	application.Phone = strings.TrimSpace(application.Phone)
	application.ResumeURL = strings.TrimSpace(application.ResumeURL)
}

func validateJobApplication(application JobApplicationRequest) error {
	switch {
	case application.FirstName == "":
		return errors.New("first name is required")
	case application.LastName == "":
		return errors.New("last name is required")
	case application.Email == "":
		return errors.New("email is required")
	case !strings.Contains(application.Email, "@"):
		return errors.New("email must be valid")
	case application.Phone == "":
		return errors.New("phone is required")
	default:
		return nil
	}
}

func validJobApplicationStatus(status string) bool {
	switch status {
	case "applied", "in_review", "rejected":
		return true
	default:
		return false
	}
}

func parseQueryInt32(r *http.Request, key string, fallback int32, min int32, max int32) int32 {
	value := strings.TrimSpace(r.URL.Query().Get(key))
	if value == "" {
		return fallback
	}
	parsed, err := strconv.ParseInt(value, 10, 32)
	if err != nil {
		return fallback
	}
	if int32(parsed) < min {
		return min
	}
	if int32(parsed) > max {
		return max
	}
	return int32(parsed)
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}
