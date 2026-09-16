package handlers

import (
	"net/http"
	"strings"
	"time"

	"pulsepoll/backend/internal/config"
	"pulsepoll/backend/internal/middleware"
	"pulsepoll/backend/internal/models"
	"pulsepoll/backend/internal/utils"

	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

// AuthHandler serves signup, login, logout, and who-am-i endpoints.
type AuthHandler struct {
	users *mongo.Collection
	cfg   *config.Config
}

// NewAuthHandler wires the collection and config.
func NewAuthHandler(db *mongo.Database, cfg *config.Config) *AuthHandler {
	return &AuthHandler{users: db.Collection("users"), cfg: cfg}
}

// req-signup
type signupReq struct {
	Name     string `json:"name"     binding:"required,min=1,max=60"`
	Email    string `json:"email"    binding:"required,email"`
	Password string `json:"password" binding:"required,min=6"`
}

// Signup creates a new account and sets a JWT cookie.
func (h *AuthHandler) Signup(c *gin.Context) {
	var req signupReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	req.Email = strings.ToLower(strings.TrimSpace(req.Email))
	req.Name = strings.TrimSpace(req.Name)

	// Bail out early if the email is taken.
	var existing models.User
	err := h.users.FindOne(c.Request.Context(), bson.M{"email": req.Email}).Decode(&existing)
	if err == nil {
		c.JSON(http.StatusConflict, gin.H{"error": "email already registered"})
		return
	}
	if err != mongo.ErrNoDocuments {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	hash, err := utils.HashPassword(req.Password)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not hash password"})
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
			c.JSON(http.StatusConflict, gin.H{"error": "email already registered"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	token, err := middleware.MakeToken(user.ID, user.Email, h.cfg)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not create token"})
		return
	}
	middleware.SetAuthCookie(c, token, h.cfg)
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
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	req.Email = strings.ToLower(strings.TrimSpace(req.Email))

	var user models.User
	if err := h.users.FindOne(c.Request.Context(), bson.M{"email": req.Email}).Decode(&user); err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid email or password"})
		return
	}
	if !utils.CheckPassword(user.PasswordHash, req.Password) {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid email or password"})
		return
	}
	token, err := middleware.MakeToken(user.ID, user.Email, h.cfg)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not create token"})
		return
	}
	middleware.SetAuthCookie(c, token, h.cfg)
	c.JSON(http.StatusOK, gin.H{
		"id":    user.ID,
		"name":  user.Name,
		"email": user.Email,
	})
}

// Logout clears the session cookie.
func (h *AuthHandler) Logout(c *gin.Context) {
	middleware.ClearAuthCookie(c, h.cfg)
	c.JSON(http.StatusOK, gin.H{"message": "logged out"})
}

// Me returns the currently authenticated user, or 401.
func (h *AuthHandler) Me(c *gin.Context) {
	userID, _ := c.Get("user_id")
	var user models.User
	if err := h.users.FindOne(c.Request.Context(), bson.M{"_id": userID}).Decode(&user); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "user not found"})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"id":    user.ID,
		"name":  user.Name,
		"email": user.Email,
	})
}
