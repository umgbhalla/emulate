package discord

import (
	"bytes"
	"crypto/ed25519"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	corehttp "github.com/vercel-labs/emulate/internal/core/http"
	corestore "github.com/vercel-labs/emulate/internal/core/store"
)

const defaultDiscordInteractionPrivateKeySeedHex = "000102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f"

func (s *Service) registerInteractionSimulationRoutes(router *corehttp.Router) {
	router.Get("/_emulate/discord/application", s.handleEmulateDiscordApplication)
	router.Post("/_emulate/discord/interactions", s.handleEmulateDiscordInteraction)
}

func (s *Service) handleEmulateDiscordApplication(c *corehttp.Context) {
	app := firstRecord(s.store.Applications.All())
	if app == nil {
		discordError(c, http.StatusNotFound, "Unknown Application", 10002)
		return
	}
	channel := firstRecord(s.store.Channels.All())
	guild := firstRecord(s.store.Guilds.All())
	user := s.findFirstNonBotUser()
	if user == nil {
		user = s.findFirstBot()
	}
	c.JSON(http.StatusOK, map[string]any{
		"application_id": stringField(app, "application_id"),
		"client_id":      stringField(app, "client_id"),
		"public_key":     stringField(app, "public_key"),
		"guild_id":       stringField(guild, "guild_id"),
		"channel_id":     stringField(channel, "channel_id"),
		"user_id":        stringField(user, "user_id"),
	})
}

func (s *Service) handleEmulateDiscordInteraction(c *corehttp.Context) {
	body := parseDiscordBody(c.Request)
	targetURL := strings.TrimSpace(stringValue(body["target_url"]))
	if !validInteractionTargetURL(targetURL) {
		c.JSON(http.StatusBadRequest, map[string]any{"error": "target_url is required"})
		return
	}
	app := s.interactionApplication(stringValue(body["application_id"]))
	if app == nil {
		discordError(c, http.StatusNotFound, "Unknown Application", 10002)
		return
	}
	privateKey, publicKey, ok := discordInteractionPrivateKey(app)
	if !ok {
		c.JSON(http.StatusBadRequest, map[string]any{"error": "discord application private_key is required for signed interactions"})
		return
	}
	channel := s.interactionChannel(stringValue(body["channel_id"]))
	if channel == nil {
		discordError(c, http.StatusNotFound, "Unknown Channel", 10003)
		return
	}
	user := s.interactionUser(stringValue(body["user_id"]))
	if user == nil {
		discordError(c, http.StatusNotFound, "Unknown User", 10013)
		return
	}

	interactionID := stringValue(body["id"])
	if interactionID == "" {
		interactionID = generateDiscordID()
	}
	token := stringValue(body["token"])
	if token == "" {
		token = generateDiscordToken()
	}
	commandName := strings.TrimSpace(stringValue(body["command_name"]))
	if commandName == "" {
		commandName = "ask"
	}
	options := recordSliceValue(body["options"])
	if len(options) == 0 {
		if content := strings.TrimSpace(stringValue(body["content"])); content != "" {
			options = []map[string]any{{"name": "text", "type": 3, "value": content}}
		}
	}
	applicationID := stringField(app, "application_id")
	guildID := stringField(channel, "guild_id")
	if requestedGuildID := stringValue(body["guild_id"]); requestedGuildID != "" {
		guildID = requestedGuildID
	}
	payload := map[string]any{
		"id":             interactionID,
		"application_id": applicationID,
		"type":           2,
		"token":          token,
		"version":        1,
		"guild_id":       guildID,
		"channel_id":     stringField(channel, "channel_id"),
		"member": map[string]any{
			"user":  formatUser(user),
			"roles": []string{},
		},
		"user": formatUser(user),
		"data": map[string]any{
			"id":      interactionCommandID(body, app, guildID, commandName),
			"name":    commandName,
			"type":    1,
			"options": options,
		},
	}
	rawPayload, err := json.Marshal(payload)
	if err != nil {
		c.JSON(http.StatusInternalServerError, map[string]any{"error": "failed to encode interaction payload"})
		return
	}
	timestamp := stringValue(body["timestamp"])
	if timestamp == "" {
		timestamp = time.Now().UTC().Format(time.RFC3339)
	}
	signature := hex.EncodeToString(ed25519.Sign(privateKey, append([]byte(timestamp), rawPayload...)))
	s.ensureInteractionWebhook(app, channel, token)

	request, err := http.NewRequest(http.MethodPost, targetURL, bytes.NewReader(rawPayload))
	if err != nil {
		c.JSON(http.StatusBadRequest, map[string]any{"error": "invalid target_url"})
		return
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("X-Signature-Ed25519", signature)
	request.Header.Set("X-Signature-Timestamp", timestamp)
	response, err := (&http.Client{Timeout: 15 * time.Second}).Do(request)
	if err != nil {
		c.JSON(http.StatusBadGateway, map[string]any{"error": err.Error()})
		return
	}
	defer response.Body.Close()
	responseBody, _ := io.ReadAll(io.LimitReader(response.Body, 1<<20))
	c.JSON(http.StatusOK, map[string]any{
		"ok":              response.StatusCode >= 200 && response.StatusCode < 300,
		"status":          response.StatusCode,
		"response_body":   string(responseBody),
		"interaction_id":  interactionID,
		"interaction_key": "discord:" + guildID + ":" + stringField(channel, "channel_id") + ":" + interactionID,
		"application_id":  applicationID,
		"token":           token,
		"public_key":      publicKey,
	})
}

func validInteractionTargetURL(value string) bool {
	if value == "" {
		return false
	}
	parsed, err := url.Parse(value)
	return err == nil && parsed.Scheme != "" && parsed.Host != ""
}

func (s *Service) interactionApplication(applicationID string) corestore.Record {
	if applicationID != "" {
		if app := firstRecord(s.store.Applications.FindBy("application_id", applicationID)); app != nil {
			return app
		}
		if app := firstRecord(s.store.Applications.FindBy("client_id", applicationID)); app != nil {
			return app
		}
	}
	return firstRecord(s.store.Applications.All())
}

func (s *Service) interactionChannel(channelID string) corestore.Record {
	if channelID != "" {
		return s.findChannel(channelID)
	}
	return firstRecord(s.store.Channels.All())
}

func (s *Service) interactionUser(userID string) corestore.Record {
	if userID != "" {
		return s.findUser(userID)
	}
	if user := s.findFirstNonBotUser(); user != nil {
		return user
	}
	return s.findFirstBot()
}

func interactionCommandID(body map[string]any, app corestore.Record, guildID string, commandName string) string {
	if commandID := stringValue(body["command_id"]); commandID != "" {
		return commandID
	}
	return stringField(app, "application_id") + ":" + guildID + ":" + commandName
}

func (s *Service) ensureInteractionWebhook(app corestore.Record, channel corestore.Record, token string) {
	applicationID := stringField(app, "application_id")
	if s.findWebhook(applicationID, token) != nil {
		return
	}
	s.store.Webhooks.Insert(corestore.Record{
		"webhook_id":     applicationID,
		"token":          token,
		"name":           stringField(app, "name"),
		"avatar":         nil,
		"channel_id":     stringField(channel, "channel_id"),
		"guild_id":       stringField(channel, "guild_id"),
		"application_id": applicationID,
		"user_id":        stringField(app, "bot_id"),
		"type":           1,
	})
}

func discordInteractionPrivateKey(app corestore.Record) (ed25519.PrivateKey, string, bool) {
	privateHex := strings.TrimSpace(stringField(app, "private_key"))
	if privateHex == "" {
		return nil, "", false
	}
	publicKey, ok := discordPublicKeyFromPrivateHex(privateHex)
	if !ok {
		return nil, "", false
	}
	privateKey, ok := discordPrivateKeyFromHex(privateHex)
	return privateKey, publicKey, ok
}

func discordPrivateKeyFromHex(privateHex string) (ed25519.PrivateKey, bool) {
	raw, err := hex.DecodeString(strings.TrimSpace(privateHex))
	if err != nil {
		return nil, false
	}
	if len(raw) == ed25519.SeedSize {
		return ed25519.NewKeyFromSeed(raw), true
	}
	if len(raw) == ed25519.PrivateKeySize {
		return ed25519.PrivateKey(raw), true
	}
	return nil, false
}

func discordPublicKeyFromPrivateHex(privateHex string) (string, bool) {
	privateKey, ok := discordPrivateKeyFromHex(privateHex)
	if !ok {
		return "", false
	}
	publicKey, ok := privateKey.Public().(ed25519.PublicKey)
	if !ok {
		return "", false
	}
	return hex.EncodeToString(publicKey), true
}

func defaultDiscordInteractionPublicKey() string {
	publicKey, _ := discordPublicKeyFromPrivateHex(defaultDiscordInteractionPrivateKeySeedHex)
	return publicKey
}

func (s *Service) findFirstNonBotUser() corestore.Record {
	for _, user := range s.store.Users.All() {
		if !boolField(user, "bot") {
			return user
		}
	}
	return nil
}
