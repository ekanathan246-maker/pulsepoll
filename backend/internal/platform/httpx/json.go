package httpx

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"

	"github.com/gin-gonic/gin"
)

var ErrInvalidJSON = errors.New("invalid JSON body")

// DecodeJSON enforces a small body, a single JSON value, and rejects unknown
// fields so typos never silently change product behavior.
func DecodeJSON(c *gin.Context, target any, maxBytes int64) error {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxBytes)
	decoder := json.NewDecoder(c.Request.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return errors.Join(ErrInvalidJSON, err)
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return ErrInvalidJSON
	}
	return nil
}
