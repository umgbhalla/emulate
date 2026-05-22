package discord

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	corehttp "github.com/vercel-labs/emulate/internal/core/http"
)

func newDiscordTestHandler() http.Handler {
	router := corehttp.NewRouter()
	Register(router, Options{})
	router.NotFound(func(c *corehttp.Context) {
		c.JSON(http.StatusNotFound, map[string]any{"message": "Not Found"})
	})
	return router
}

func discordRequest(handler http.Handler, method string, path string, body string) (*httptest.ResponseRecorder, map[string]any) {
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bot test-token")
	res := httptest.NewRecorder()
	handler.ServeHTTP(res, req)
	var parsed map[string]any
	if strings.Contains(res.Header().Get("Content-Type"), "application/json") && res.Body.Len() > 0 {
		_ = json.Unmarshal(res.Body.Bytes(), &parsed)
	}
	return res, parsed
}

func TestDiscordBotClientMessageLifecycle(t *testing.T) {
	handler := newDiscordTestHandler()

	res, body := discordRequest(handler, http.MethodGet, "/api/v10/users/@me", "")
	if res.Code != http.StatusOK || body["username"] != "emulate-bot" || body["bot"] != true {
		t.Fatalf("unexpected current user: status=%d body=%s", res.Code, res.Body.String())
	}

	res, _ = discordRequest(handler, http.MethodGet, "/api/v10/guilds/100000000000000001/channels", "")
	if res.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", res.Code, res.Body.String())
	}
	var channels []map[string]any
	if err := json.Unmarshal(res.Body.Bytes(), &channels); err != nil {
		t.Fatal(err)
	}
	if len(channels) < 1 || channels[0]["name"] != "general" {
		t.Fatalf("unexpected channels: %#v", channels)
	}

	res, body = discordRequest(handler, http.MethodPost, "/api/v10/channels/300000000000000001/messages", "{\"content\":\"deploy succeeded\"}")
	if res.Code != http.StatusOK || body["content"] != "deploy succeeded" {
		t.Fatalf("unexpected create message: status=%d body=%s", res.Code, res.Body.String())
	}
	messageID := body["id"].(string)

	res, body = discordRequest(handler, http.MethodPatch, "/api/v10/channels/300000000000000001/messages/"+messageID, "{\"content\":\"deploy verified\"}")
	if res.Code != http.StatusOK || body["content"] != "deploy verified" || body["edited_timestamp"] == nil {
		t.Fatalf("unexpected update message: status=%d body=%s", res.Code, res.Body.String())
	}

	res, _ = discordRequest(handler, http.MethodGet, "/api/v10/channels/300000000000000001/messages", "")
	if res.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", res.Code, res.Body.String())
	}
	var messages []map[string]any
	if err := json.Unmarshal(res.Body.Bytes(), &messages); err != nil {
		t.Fatal(err)
	}
	if len(messages) != 1 || messages[0]["id"] != messageID {
		t.Fatalf("unexpected messages: %#v", messages)
	}

	res, _ = discordRequest(handler, http.MethodDelete, "/api/v10/channels/300000000000000001/messages/"+messageID, "")
	if res.Code != http.StatusNoContent {
		t.Fatalf("status = %d, body = %s", res.Code, res.Body.String())
	}
}

func TestDiscordRejectsUnauthorizedAndEmptyMessages(t *testing.T) {
	handler := newDiscordTestHandler()

	req := httptest.NewRequest(http.MethodGet, "/api/v10/users/@me", nil)
	res := httptest.NewRecorder()
	handler.ServeHTTP(res, req)
	if res.Code != http.StatusUnauthorized || !strings.Contains(res.Body.String(), "Unauthorized") {
		t.Fatalf("unexpected unauthorized response: status=%d body=%s", res.Code, res.Body.String())
	}

	res, body := discordRequest(handler, http.MethodPost, "/api/v10/channels/300000000000000001/messages", "{}")
	if res.Code != http.StatusBadRequest || body["code"] != float64(50006) {
		t.Fatalf("unexpected empty message response: status=%d body=%#v", res.Code, body)
	}
}

func TestDiscordSeedFromConfig(t *testing.T) {
	router := corehttp.NewRouter()
	Register(router, Options{Seed: &SeedConfig{
		Guild: &GuildSeed{Name: "Acme"},
		Bot:   &BotSeed{Username: "acme-bot", Token: "acme-token"},
		Channels: []ChannelSeed{
			{Name: "ops", Topic: "Ops alerts"},
		},
	}})

	listReq := httptest.NewRequest(http.MethodGet, "/api/v10/guilds/100000000000000001/channels", nil)
	listReq.Header.Set("Authorization", "Bot acme-token")
	listRes := httptest.NewRecorder()
	router.ServeHTTP(listRes, listReq)
	if listRes.Code != http.StatusOK || !strings.Contains(listRes.Body.String(), "\"name\":\"ops\"") {
		t.Fatalf("unexpected seeded channels: status=%d body=%s", listRes.Code, listRes.Body.String())
	}
}

func TestDiscordInspector(t *testing.T) {
	handler := newDiscordTestHandler()
	res, _ := discordRequest(handler, http.MethodPost, "/api/v10/channels/300000000000000001/messages", "{\"content\":\"visible in inspector\"}")
	if res.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", res.Code, res.Body.String())
	}

	req := httptest.NewRequest(http.MethodGet, "/discord", nil)
	page := httptest.NewRecorder()
	handler.ServeHTTP(page, req)
	if page.Code != http.StatusOK || !strings.Contains(page.Body.String(), "Message Inspector") || !strings.Contains(page.Body.String(), "visible in inspector") {
		t.Fatalf("unexpected inspector: status=%d body=%s", page.Code, page.Body.String())
	}
}
