package httpx_test

import (
	"net/http/httptest"
	"strings"
	"testing"

	"pulsepoll/backend/internal/platform/httpx"

	"github.com/gin-gonic/gin"
)

func TestDecodeJSONRejectsUnknownFields(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest("POST", "/", strings.NewReader(`{"known":"ok","typo":true}`))

	var body struct {
		Known string `json:"known"`
	}
	if err := httpx.DecodeJSON(ctx, &body, 1024); err == nil {
		t.Fatal("DecodeJSON() accepted an unknown field")
	}
}

func TestDecodeJSONRejectsOversizedBodies(t *testing.T) {
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest("POST", "/", strings.NewReader(`{"known":"too long"}`))

	var body struct {
		Known string `json:"known"`
	}
	if err := httpx.DecodeJSON(ctx, &body, 8); err == nil {
		t.Fatal("DecodeJSON() accepted an oversized body")
	}
}
