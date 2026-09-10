package bot

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestHandleGetRoomCodeRejectsInvalidConnectCode(t *testing.T) {
	gin.SetMode(gin.TestMode)
	// no Redis is touched when validation fails, so a zero Bot is sufficient
	handler := handleGetRoomCode(&Bot{RedisInterface: &RedisInterface{}})

	for _, code := range []string{"", "short", "waytoolongcode"} {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodGet, "/game/roomcode?connectCode="+code, nil)

		handler(c)

		if w.Code != http.StatusBadRequest {
			t.Errorf("connectCode %q: status = %d, want %d", code, w.Code, http.StatusBadRequest)
		}
		var body HttpError
		if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
			t.Fatalf("connectCode %q: response is not an HttpError: %v", code, err)
		}
		if body.Error != "invalid connect code" {
			t.Errorf("connectCode %q: error = %q", code, body.Error)
		}
	}
}
