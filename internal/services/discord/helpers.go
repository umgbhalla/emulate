package discord

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	corehttp "github.com/vercel-labs/emulate/internal/core/http"
	corestore "github.com/vercel-labs/emulate/internal/core/store"
)

var snowflakeState = struct {
	sync.Mutex
	millisecond int64
	counter     int64
}{}

func generateDiscordID() string {
	now := time.Now().UnixMilli()
	snowflakeState.Lock()
	defer snowflakeState.Unlock()
	if now != snowflakeState.millisecond {
		snowflakeState.millisecond = now
		snowflakeState.counter = 0
	}
	snowflakeState.counter++
	return strconv.FormatInt((now-1420070400000)<<22|(snowflakeState.counter&0x3ff), 10)
}

func generateDiscordToken() string {
	raw := make([]byte, 16)
	if _, err := rand.Read(raw); err != nil {
		return "bot-" + strconv.FormatInt(time.Now().UnixNano(), 36)
	}
	return "bot-" + hex.EncodeToString(raw)
}

func parseDiscordBody(r *http.Request) map[string]any {
	raw, _ := io.ReadAll(r.Body)
	if len(strings.TrimSpace(string(raw))) == 0 {
		return map[string]any{}
	}
	var body map[string]any
	if err := json.Unmarshal(raw, &body); err != nil || body == nil {
		return map[string]any{}
	}
	return body
}

func (s *Service) authenticatedUser(c *corehttp.Context) (corestore.Record, bool) {
	token := discordAuthToken(c.Header("Authorization"))
	if token == "" {
		discordError(c, http.StatusUnauthorized, "401: Unauthorized", 0)
		return nil, false
	}
	tokenRecord := firstRecord(s.store.Tokens.FindBy("token", token))
	if tokenRecord == nil {
		discordError(c, http.StatusUnauthorized, "401: Unauthorized", 0)
		return nil, false
	}
	user := s.findUser(stringField(tokenRecord, "user_id"))
	if user == nil {
		discordError(c, http.StatusUnauthorized, "401: Unauthorized", 0)
		return nil, false
	}
	return user, true
}

func discordAuthToken(value string) string {
	value = strings.TrimSpace(value)
	for _, prefix := range []string{"Bot ", "bot ", "Bearer ", "bearer ", "token ", "Token "} {
		if strings.HasPrefix(value, prefix) {
			return strings.TrimSpace(strings.TrimPrefix(value, prefix))
		}
	}
	return value
}

func discordError(c *corehttp.Context, status int, message string, code int) {
	c.JSON(status, map[string]any{"message": message, "code": code})
}

func firstRecord(records []corestore.Record) corestore.Record {
	if len(records) == 0 {
		return nil
	}
	return records[0]
}

func stringField(record corestore.Record, field string) string {
	if record == nil {
		return ""
	}
	return stringValue(record[field])
}

func intField(record corestore.Record, field string) int {
	if record == nil {
		return 0
	}
	switch value := record[field].(type) {
	case int:
		return value
	case int64:
		return int(value)
	case float64:
		return int(value)
	case json.Number:
		n, _ := value.Int64()
		return int(n)
	default:
		n, _ := strconv.Atoi(fmt.Sprint(value))
		return n
	}
}

func boolField(record corestore.Record, field string) bool {
	if record == nil {
		return false
	}
	switch value := record[field].(type) {
	case bool:
		return value
	case string:
		return value == "true"
	default:
		return false
	}
}

func stringValue(value any) string {
	switch v := value.(type) {
	case string:
		return v
	case fmt.Stringer:
		return v.String()
	case nil:
		return ""
	default:
		return fmt.Sprint(v)
	}
}

func intValue(value any) int {
	switch v := value.(type) {
	case int:
		return v
	case int64:
		return int(v)
	case float64:
		return int(v)
	case string:
		n, _ := strconv.Atoi(v)
		return n
	default:
		n, _ := strconv.Atoi(fmt.Sprint(v))
		return n
	}
}

func recordSliceValue(value any) []map[string]any {
	switch v := value.(type) {
	case []map[string]any:
		return append([]map[string]any(nil), v...)
	case []any:
		out := make([]map[string]any, 0, len(v))
		for _, item := range v {
			if record, ok := item.(map[string]any); ok {
				out = append(out, record)
			}
		}
		return out
	default:
		return nil
	}
}
