package handlers

import (
	"database/sql"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"

	"github.com/alicenstar/astrid/internal/auth"
	"github.com/alicenstar/astrid/internal/models"
)

type AuthHandler struct {
	db           *sql.DB
	rdb          *redis.Client
	tmpl         *Templates
	oauthConfig  *oauth2.Config
	oauthEnabled bool
}

func NewAuthHandler(db *sql.DB, rdb *redis.Client, tmpl *Templates, googleClientID, googleSecret, googleRedirectURL string) *AuthHandler {
	h := &AuthHandler{db: db, rdb: rdb, tmpl: tmpl}
	if googleClientID != "" && googleSecret != "" {
		h.oauthEnabled = true
		h.oauthConfig = &oauth2.Config{
			ClientID:     googleClientID,
			ClientSecret: googleSecret,
			RedirectURL:  googleRedirectURL,
			Scopes:       []string{"openid", "email", "profile"},
			Endpoint:     google.Endpoint,
		}
	}
	return h
}

func (h *AuthHandler) setSessionCookie(w http.ResponseWriter, sessionID string) {
	http.SetCookie(w, &http.Cookie{
		Name:     "astrid_session",
		Value:    sessionID,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   7 * 24 * 60 * 60,
	})
}

func (h *AuthHandler) LoginPage(w http.ResponseWriter, r *http.Request) {
	data := map[string]any{
		"Title":         "Log In",
		"ActiveNav":     "",
		"GoogleEnabled": h.oauthEnabled,
	}
	h.tmpl.Render(w, "login", data)
}

func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	email := strings.TrimSpace(r.FormValue("email"))
	password := r.FormValue("password")

	user, err := models.FindByEmail(h.db, email)
	if err != nil {
		h.tmpl.RenderError(w, "Something went wrong", http.StatusInternalServerError)
		return
	}
	if user == nil || !user.ValidatePassword(password) {
		data := map[string]any{
			"Title":         "Log In",
			"ActiveNav":     "",
			"GoogleEnabled": h.oauthEnabled,
			"Error":         "Invalid email or password.",
			"Email":         email,
		}
		h.tmpl.Render(w, "login", data)
		return
	}

	sessionID, err := auth.CreateSession(h.rdb, user.ID, user.Email)
	if err != nil {
		h.tmpl.RenderError(w, "Could not create session", http.StatusInternalServerError)
		return
	}
	h.setSessionCookie(w, sessionID)
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func (h *AuthHandler) SignupPage(w http.ResponseWriter, r *http.Request) {
	data := map[string]any{
		"Title":         "Sign Up",
		"ActiveNav":     "",
		"GoogleEnabled": h.oauthEnabled,
	}
	h.tmpl.Render(w, "signup", data)
}

func (h *AuthHandler) Signup(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	name := strings.TrimSpace(r.FormValue("name"))
	email := strings.TrimSpace(r.FormValue("email"))
	password := r.FormValue("password")

	var validationErr string
	if name == "" {
		validationErr = "Name is required."
	} else if email == "" || !strings.Contains(email, "@") {
		validationErr = "A valid email is required."
	} else if len(password) < 8 {
		validationErr = "Password must be at least 8 characters."
	}

	if validationErr != "" {
		data := map[string]any{
			"Title":         "Sign Up",
			"ActiveNav":     "",
			"GoogleEnabled": h.oauthEnabled,
			"Error":         validationErr,
			"Name":          name,
			"Email":         email,
		}
		h.tmpl.Render(w, "signup", data)
		return
	}

	user, err := models.CreateUser(h.db, name, email, password)
	if err != nil {
		if strings.Contains(err.Error(), "unique") || strings.Contains(err.Error(), "duplicate") {
			data := map[string]any{
				"Title":         "Sign Up",
				"ActiveNav":     "",
				"GoogleEnabled": h.oauthEnabled,
				"Error":         "An account with this email already exists.",
				"Name":          name,
				"Email":         email,
			}
			h.tmpl.Render(w, "signup", data)
			return
		}
		h.tmpl.RenderError(w, "Could not create account", http.StatusInternalServerError)
		return
	}

	sessionID, _ := auth.CreateSession(h.rdb, user.ID, user.Email)
	h.setSessionCookie(w, sessionID)
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func (h *AuthHandler) DemoLogin(w http.ResponseWriter, r *http.Request) {
	user, err := models.EnsureDefaultUser(h.db)
	if err != nil {
		log.Printf("DemoLogin: EnsureDefaultUser failed: %v", err)
		h.tmpl.RenderError(w, "Could not create demo session", http.StatusInternalServerError)
		return
	}
	if err := models.SeedDemoData(h.db, user.ID); err != nil {
		log.Printf("DemoLogin: SeedDemoData failed: %v", err)
		h.tmpl.RenderError(w, "Could not set up demo data", http.StatusInternalServerError)
		return
	}
	// Invalidate cached daily summaries for seeded dates
	today := time.Now()
	for i := 0; i < 5; i++ {
		models.InvalidateDailyCache(h.rdb, user.ID, today.AddDate(0, 0, -i))
	}
	models.InvalidateStreakCache(h.rdb, user.ID)
	sessionID, _ := auth.CreateSession(h.rdb, user.ID, user.Email)
	h.setSessionCookie(w, sessionID)
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func (h *AuthHandler) Logout(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie("astrid_session")
	if err == nil {
		auth.DeleteSession(h.rdb, cookie.Value)
	}
	http.SetCookie(w, &http.Cookie{
		Name:     "astrid_session",
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		MaxAge:   -1,
	})
	http.Redirect(w, r, "/login", http.StatusSeeOther)
}

func (h *AuthHandler) GoogleLogin(w http.ResponseWriter, r *http.Request) {
	if !h.oauthEnabled {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}
	url := h.oauthConfig.AuthCodeURL("state")
	http.Redirect(w, r, url, http.StatusTemporaryRedirect)
}

func (h *AuthHandler) GoogleCallback(w http.ResponseWriter, r *http.Request) {
	if !h.oauthEnabled {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	code := r.URL.Query().Get("code")
	token, err := h.oauthConfig.Exchange(r.Context(), code)
	if err != nil {
		h.tmpl.RenderError(w, "Google authentication failed", http.StatusBadRequest)
		return
	}

	client := h.oauthConfig.Client(r.Context(), token)
	resp, err := client.Get("https://www.googleapis.com/oauth2/v2/userinfo")
	if err != nil {
		h.tmpl.RenderError(w, "Could not fetch Google profile", http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	var gUser struct {
		ID    string `json:"id"`
		Email string `json:"email"`
		Name  string `json:"name"`
	}
	json.Unmarshal(body, &gUser)

	user, err := models.FindOrCreateGoogleUser(h.db, gUser.ID, gUser.Email, gUser.Name)
	if err != nil {
		h.tmpl.RenderError(w, "Could not create account", http.StatusInternalServerError)
		return
	}

	sessionID, _ := auth.CreateSession(h.rdb, user.ID, user.Email)
	h.setSessionCookie(w, sessionID)
	http.Redirect(w, r, "/", http.StatusSeeOther)
}
