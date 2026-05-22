package discord

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
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

func TestDiscordBotRestHelpers(t *testing.T) {
	handler := newDiscordTestHandler()

	res, body := discordRequest(handler, http.MethodGet, "/api/v10/gateway/bot", "")
	if res.Code != http.StatusOK || body["url"] == "" || body["shards"] != float64(1) {
		t.Fatalf("unexpected gateway bot response: status=%d body=%s", res.Code, res.Body.String())
	}

	res, body = discordRequest(handler, http.MethodPost, "/api/v10/guilds/100000000000000001/channels", "{\"name\":\"alerts\",\"topic\":\"Deploy alerts\"}")
	if res.Code != http.StatusCreated || body["name"] != "alerts" {
		t.Fatalf("unexpected create channel response: status=%d body=%s", res.Code, res.Body.String())
	}
	channelID := body["id"].(string)

	res, body = discordRequest(handler, http.MethodPost, "/api/v10/channels/"+channelID+"/messages", "{\"content\":\"ship it\"}")
	if res.Code != http.StatusOK {
		t.Fatalf("unexpected create message response: status=%d body=%s", res.Code, res.Body.String())
	}
	messageID := body["id"].(string)

	res, body = discordRequest(handler, http.MethodGet, "/api/v10/channels/"+channelID+"/messages/"+messageID, "")
	if res.Code != http.StatusOK || body["content"] != "ship it" {
		t.Fatalf("unexpected get message response: status=%d body=%s", res.Code, res.Body.String())
	}

	res, _ = discordRequest(handler, http.MethodPut, "/api/v10/channels/"+channelID+"/messages/"+messageID+"/reactions/%F0%9F%9A%80/@me", "")
	if res.Code != http.StatusNoContent {
		t.Fatalf("unexpected add reaction response: status=%d body=%s", res.Code, res.Body.String())
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v10/channels/"+channelID+"/messages/"+messageID+"/reactions/%F0%9F%9A%80", nil)
	req.Header.Set("Authorization", "Bot test-token")
	res = httptest.NewRecorder()
	handler.ServeHTTP(res, req)
	if res.Code != http.StatusOK || !strings.Contains(res.Body.String(), "emulate-bot") {
		t.Fatalf("unexpected reaction users response: status=%d body=%s", res.Code, res.Body.String())
	}

	res, _ = discordRequest(handler, http.MethodPut, "/api/v10/channels/"+channelID+"/pins/"+messageID, "")
	if res.Code != http.StatusNoContent {
		t.Fatalf("unexpected pin response: status=%d body=%s", res.Code, res.Body.String())
	}
	req = httptest.NewRequest(http.MethodGet, "/api/v10/channels/"+channelID+"/pins", nil)
	req.Header.Set("Authorization", "Bot test-token")
	res = httptest.NewRecorder()
	handler.ServeHTTP(res, req)
	if res.Code != http.StatusOK || !strings.Contains(res.Body.String(), messageID) {
		t.Fatalf("unexpected pins response: status=%d body=%s", res.Code, res.Body.String())
	}

	res, _ = discordRequest(handler, http.MethodPost, "/api/v10/channels/"+channelID+"/typing", "")
	if res.Code != http.StatusNoContent {
		t.Fatalf("unexpected typing response: status=%d body=%s", res.Code, res.Body.String())
	}

	res, _ = discordRequest(handler, http.MethodPost, "/api/v10/channels/"+channelID+"/messages/bulk-delete", "{\"messages\":[\""+messageID+"\"]}")
	if res.Code != http.StatusNoContent {
		t.Fatalf("unexpected bulk delete response: status=%d body=%s", res.Code, res.Body.String())
	}
}

func TestDiscordGuildLifecycle(t *testing.T) {
	handler := newDiscordTestHandler()

	res, body := discordRequest(handler, http.MethodPost, "/api/v10/guilds", "{\"name\":\"Created Guild\"}")
	if res.Code != http.StatusCreated || body["name"] != "Created Guild" {
		t.Fatalf("unexpected create guild response: status=%d body=%s", res.Code, res.Body.String())
	}
	guildID := body["id"].(string)

	res, body = discordRequest(handler, http.MethodPatch, "/api/v10/guilds/"+guildID, "{\"name\":\"Renamed Guild\"}")
	if res.Code != http.StatusOK || body["name"] != "Renamed Guild" {
		t.Fatalf("unexpected update guild response: status=%d body=%s", res.Code, res.Body.String())
	}

	res, _ = discordRequest(handler, http.MethodPost, "/api/v10/guilds/"+guildID+"/channels", "{\"name\":\"guild-local\"}")
	if res.Code != http.StatusCreated {
		t.Fatalf("unexpected create guild channel response: status=%d body=%s", res.Code, res.Body.String())
	}

	res, _ = discordRequest(handler, http.MethodDelete, "/api/v10/guilds/"+guildID, "")
	if res.Code != http.StatusNoContent {
		t.Fatalf("unexpected delete guild response: status=%d body=%s", res.Code, res.Body.String())
	}

	res, _ = discordRequest(handler, http.MethodGet, "/api/v10/guilds/"+guildID, "")
	if res.Code != http.StatusNotFound {
		t.Fatalf("expected deleted guild to be gone: status=%d body=%s", res.Code, res.Body.String())
	}
}

func TestDiscordRolesWebhooksAndApplicationCommands(t *testing.T) {
	handler := newDiscordTestHandler()

	res, body := discordRequest(handler, http.MethodPost, "/api/v10/guilds/100000000000000001/roles", "{\"name\":\"Deployers\",\"permissions\":\"8\",\"mentionable\":true}")
	if res.Code != http.StatusOK || body["name"] != "Deployers" {
		t.Fatalf("unexpected create role response: status=%d body=%s", res.Code, res.Body.String())
	}
	roleID := body["id"].(string)

	res, _ = discordRequest(handler, http.MethodPut, "/api/v10/guilds/100000000000000001/members/200000000000000002/roles/"+roleID, "")
	if res.Code != http.StatusNoContent {
		t.Fatalf("unexpected add member role response: status=%d body=%s", res.Code, res.Body.String())
	}
	res, body = discordRequest(handler, http.MethodGet, "/api/v10/guilds/100000000000000001/members/200000000000000002", "")
	if res.Code != http.StatusOK || !strings.Contains(res.Body.String(), roleID) {
		t.Fatalf("unexpected member role response: status=%d body=%s", res.Code, res.Body.String())
	}

	res, body = discordRequest(handler, http.MethodPost, "/api/v10/applications/900000000000000001/guilds/100000000000000001/commands", "{\"name\":\"deploy\",\"description\":\"Deploy command\",\"type\":1}")
	if res.Code != http.StatusCreated || body["name"] != "deploy" {
		t.Fatalf("unexpected create command response: status=%d body=%s", res.Code, res.Body.String())
	}
	commandID := body["id"].(string)
	res, body = discordRequest(handler, http.MethodPatch, "/api/v10/applications/900000000000000001/guilds/100000000000000001/commands/"+commandID, "{\"description\":\"Deploy now\"}")
	if res.Code != http.StatusOK || body["description"] != "Deploy now" {
		t.Fatalf("unexpected update command response: status=%d body=%s", res.Code, res.Body.String())
	}

	res, body = discordRequest(handler, http.MethodPost, "/api/v10/channels/300000000000000001/webhooks", "{\"name\":\"CI\"}")
	if res.Code != http.StatusOK || body["name"] != "CI" {
		t.Fatalf("unexpected create webhook response: status=%d body=%s", res.Code, res.Body.String())
	}
	webhookID := body["id"].(string)
	webhookToken := body["token"].(string)

	res, body = discordRequest(handler, http.MethodPost, "/api/v10/webhooks/"+webhookID+"/"+webhookToken+"?wait=true", "{\"content\":\"from webhook\"}")
	if res.Code != http.StatusOK || body["content"] != "from webhook" || body["webhook_id"] != webhookID {
		t.Fatalf("unexpected execute webhook response: status=%d body=%s", res.Code, res.Body.String())
	}
	messageID := body["id"].(string)

	res, body = discordRequest(handler, http.MethodPatch, "/api/v10/webhooks/"+webhookID+"/"+webhookToken+"/messages/"+messageID, "{\"content\":\"edited webhook\"}")
	if res.Code != http.StatusOK || body["content"] != "edited webhook" {
		t.Fatalf("unexpected update webhook message response: status=%d body=%s", res.Code, res.Body.String())
	}
}

func TestDiscordOAuthAuthorizationCodeFlow(t *testing.T) {
	handler := newDiscordTestHandler()
	redirectURI := "http://localhost:3000/api/auth/callback/discord"

	authorize := httptest.NewRequest(http.MethodGet, "/oauth2/authorize?client_id=discord-client-id&redirect_uri="+url.QueryEscape(redirectURI)+"&scope=identify%20guilds&state=abc", nil)
	authorizeRes := httptest.NewRecorder()
	handler.ServeHTTP(authorizeRes, authorize)
	if authorizeRes.Code != http.StatusOK || !strings.Contains(authorizeRes.Body.String(), "Sign in to Discord") || !strings.Contains(authorizeRes.Body.String(), "developer") {
		t.Fatalf("unexpected authorize page: status=%d body=%s", authorizeRes.Code, authorizeRes.Body.String())
	}

	form := url.Values{
		"user_id":      {"200000000000000002"},
		"client_id":    {"discord-client-id"},
		"redirect_uri": {redirectURI},
		"scope":        {"identify guilds"},
		"state":        {"abc"},
	}
	callback := httptest.NewRequest(http.MethodPost, "/oauth2/authorize/callback", strings.NewReader(form.Encode()))
	callback.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	callbackRes := httptest.NewRecorder()
	handler.ServeHTTP(callbackRes, callback)
	if callbackRes.Code != http.StatusFound {
		t.Fatalf("unexpected callback response: status=%d body=%s", callbackRes.Code, callbackRes.Body.String())
	}
	location, err := url.Parse(callbackRes.Header().Get("Location"))
	if err != nil {
		t.Fatal(err)
	}
	code := location.Query().Get("code")
	if code == "" || location.Query().Get("state") != "abc" {
		t.Fatalf("unexpected callback location: %s", callbackRes.Header().Get("Location"))
	}

	tokenForm := url.Values{
		"grant_type":    {"authorization_code"},
		"code":          {code},
		"client_id":     {"discord-client-id"},
		"client_secret": {"discord-client-secret"},
		"redirect_uri":  {redirectURI},
	}
	tokenReq := httptest.NewRequest(http.MethodPost, "/api/v10/oauth2/token", strings.NewReader(tokenForm.Encode()))
	tokenReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	tokenRes := httptest.NewRecorder()
	handler.ServeHTTP(tokenRes, tokenReq)
	var tokenBody map[string]any
	if err := json.Unmarshal(tokenRes.Body.Bytes(), &tokenBody); err != nil {
		t.Fatal(err)
	}
	if tokenRes.Code != http.StatusOK || tokenBody["token_type"] != "Bearer" || tokenBody["access_token"] == "" {
		t.Fatalf("unexpected token response: status=%d body=%s", tokenRes.Code, tokenRes.Body.String())
	}

	meReq := httptest.NewRequest(http.MethodGet, "/api/v10/users/@me", nil)
	meReq.Header.Set("Authorization", "Bearer "+tokenBody["access_token"].(string))
	meRes := httptest.NewRecorder()
	handler.ServeHTTP(meRes, meReq)
	if meRes.Code != http.StatusOK || !strings.Contains(meRes.Body.String(), "developer") {
		t.Fatalf("unexpected bearer user response: status=%d body=%s", meRes.Code, meRes.Body.String())
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
