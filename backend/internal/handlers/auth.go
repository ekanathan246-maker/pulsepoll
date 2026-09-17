package handlers

import (
	"net/http"
	"net/mail"
	"strings"
	"time"
	"unicode/utf8"

	"pulsepoll/backend/internal/auth"
	"pulsepoll/backend/internal/config"
	"pulsepoll/backend/internal/middleware"
	"pulsepoll/backend/internal/models"
	"pulsepoll/backend/internal/platform/httpx"
	"pulsepoll/backend/internal/utils"

	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

// AuthHandler serves signup, login, logout, and who-am-i endpoints.
type AuthHandler struct {
	users    *mongo.Collection
	cfg      *config.Config
	sessions *auth.Manager
}

// NewAuthHandler wires the collection and config.
func NewAuthHandler(db *mongo.Database, cfg *config.Config, sessions *auth.Manager) *AuthHandler {
	return &AuthHandler{users: db.Collection("users"), cfg: cfg, sessions: sessions}
}

// req-signup
type signupReq struct {
	Name     string `json:"name"     binding:"required,min=1,max=60"`
	Email    string `json:"email"    binding:"required,email"`
	Password string `json:"password" binding:"required,min=8,max=128"`
}

// Signup creates a new account and sets a JWT cookie.
func (h *AuthHandler) Signup(c *gin.Context) {
	var req signupReq
	if err := httpx.DecodeJSON(c, &req, 8<<10); err != nil {
		c.JSON(http.StatusBadRequest, middleware.ErrorBody("invalid_request", "Enter a valid name, email, and password of at least 8 characters."))
		return
	}
	req.Email = strings.ToLower(strings.TrimSpace(req.Email))
	req.Name = strings.TrimSpace(req.Name)
	parsedEmail, emailErr := mail.ParseAddress(req.Email)
	if utf8.RuneCountInString(req.Name) < 1 || utf8.RuneCountInString(req.Name) > 60 || emailErr != nil || parsedEmail.Address != req.Email || len(req.Password) < 8 || len(req.Password) > 128 {
		c.JSON(http.StatusBadRequest, middleware.ErrorBody("invalid_request", "Enter a valid name, email, and password of at least 8 characters."))
		return
	}

	// Bail out early if the email is taken.
	var existing models.User
	err := h.users.FindOne(c.Request.Context(), bson.M{"email": req.Email}).Decode(&existing)
	if err == nil {
		c.JSON(http.StatusConflict, middleware.ErrorBody("email_taken", "An account already uses this email."))
		return
	}
	if err != mongo.ErrNoDocuments {
		c.JSON(http.StatusInternalServerError, middleware.ErrorBody("internal_error", "Account creation is temporarily unavailable."))
		return
	}

	hash, err := utils.HashPassword(req.Password)
	if err != nil {
		c.JSON(http.StatusInternalServerError, middleware.ErrorBody("internal_error", "Account creation is temporarily unavailable."))
		return
	}

	user := models.User{
		ID:           primitive.NewObjectID(),
		Name:         req.Name,
		Email:        req.Email,
		PasswordHash: hash,
		CreatedAt:    time.Now(),
	}

	// Insert; a duplicate-email race is caught by the unique index.
	if _, err := h.users.InsertOne(c.Request.Context(), user); err != nil {
		if mongo.IsDuplicateKeyError(err) {
			c.JSON(http.StatusConflict, middleware.ErrorBody("email_taken", "An account already uses this email."))
			return
		}
		c.JSON(http.StatusInternalServerError, middleware.ErrorBody("internal_error", "Account creation is temporarily unavailable."))
		return
	}

	credentials, err := h.sessions.Create(c.Request.Context(), user.ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, middleware.ErrorBody("internal_error", "Account creation is temporarily unavailable."))
		return
	}
	middleware.SetSessionCookies(c, credentials, h.cfg)
	c.JSON(http.StatusOK, gin.H{
		"id":    user.ID,
		"name":  user.Name,
		"email": user.Email,
	})
}

// Login accepts credentials and returns a JWT cookie.
func (h *AuthHandler) Login(c *gin.Context) {
	var req struct {
		Email    string `json:"email"    binding:"required"`
		Password string `json:"password" binding:"required"`
	}
	if err := httpx.DecodeJSON(c, &req, 8<<10); err != nil {
		c.JSON(http.StatusBadRequest, middleware.ErrorBody("invalid_request", "Enter your email and password."))
		return
	}
	req.Email = strings.ToLower(strings.TrimSpace(req.Email))
	if req.Email == "" || req.Password == "" {
		c.JSON(http.StatusBadRequest, middleware.ErrorBody("invalid_request", "Enter your email and password."))
		return
	}

	var user models.User
	if err := h.users.FindOne(c.Request.Context(), bson.M{"email": req.Email}).Decode(&user); err != nil {
		c.JSON(http.StatusUnauthorized, middleware.ErrorBody("invalid_credentials", "Email or password is incorrect."))
		return
	}
	if !utils.CheckPassword(user.PasswordHash, req.Password) {
		c.JSON(http.StatusUnauthorized, middleware.ErrorBody("invalid_credentials", "Email or password is incorrect."))
		return
	}
	// Rotate any session presented by this browser after credentials succeed.
	// Revocation is deliberately after password verification to avoid turning
	// the login endpoint into a cross-site logout primitive.
	if previousToken, cookieErr := c.Cookie(h.cfg.CookieName); cookieErr == nil {
		_ = h.sessions.Revoke(c.Request.Context(), previousToken)
	}
	credentials, err := h.sessions.Create(c.Request.Context(), user.ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, middleware.ErrorBody("internal_error", "Login is temporarily unavailable."))
		return
	}
	middleware.SetSessionCookies(c, credentials, h.cfg)
	c.JSON(http.StatusOK, gin.H{
		"id":    user.ID,
		"name":  user.Name,
		"email": user.Email,
	})
}

// Logout clears the session cookie.
func (h *AuthHandler) Logout(c *gin.Context) {
	token, _ := c.Cookie(h.cfg.CookieName)
	_ = h.sessions.Revoke(c.Request.Context(), token)
	middleware.ClearSessionCookies(c, h.cfg)
	c.JSON(http.StatusOK, gin.H{"message": "logged out"})
}

// Me returns the currently authenticated user, or 401.
func (h *AuthHandler) Me(c *gin.Context) {
	userID, _ := c.Get("user_id")
	var user models.User
	if err := h.users.FindOne(c.Request.Context(), bson.M{"_id": userID}).Decode(&user); err != nil {
		c.JSON(http.StatusNotFound, middleware.ErrorBody("user_not_found", "Account not found."))
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"id":    user.ID,
		"name":  user.Name,
		"email": user.Email,
	})
}
