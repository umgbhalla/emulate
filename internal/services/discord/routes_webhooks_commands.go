package discord

import (
	"net/http"
	"sort"
	"strings"
	"time"

	corehttp "github.com/vercel-labs/emulate/internal/core/http"
	corestore "github.com/vercel-labs/emulate/internal/core/store"
)

func (s *Service) handleListChannelWebhooks(c *corehttp.Context) {
	if _, ok := s.authenticatedUser(c); !ok {
		return
	}
	channel := s.findChannel(c.Param("channelId"))
	if channel == nil {
		discordError(c, http.StatusNotFound, "Unknown Channel", 10003)
		return
	}
	webhooks := []map[string]any{}
	for _, webhook := range s.store.Webhooks.FindBy("channel_id", stringField(channel, "channel_id")) {
		webhooks = append(webhooks, s.formatWebhook(webhook, c))
	}
	c.JSON(http.StatusOK, webhooks)
}

func (s *Service) handleCreateChannelWebhook(c *corehttp.Context) {
	user, ok := s.authenticatedUser(c)
	if !ok {
		return
	}
	channel := s.findChannel(c.Param("channelId"))
	if channel == nil {
		discordError(c, http.StatusNotFound, "Unknown Channel", 10003)
		return
	}
	body := parseDiscordBody(c.Request)
	name := strings.TrimSpace(stringValue(body["name"]))
	if name == "" {
		name = "webhook"
	}
	webhook := s.store.Webhooks.Insert(corestore.Record{
		"webhook_id":     generateDiscordID(),
		"token":          generateDiscordToken(),
		"name":           name,
		"avatar":         body["avatar"],
		"channel_id":     stringField(channel, "channel_id"),
		"guild_id":       stringField(channel, "guild_id"),
		"application_id": nil,
		"user_id":        stringField(user, "user_id"),
		"type":           1,
	})
	c.JSON(http.StatusOK, s.formatWebhook(webhook, c))
}

func (s *Service) handleGetWebhook(c *corehttp.Context) {
	webhook, ok := s.requestWebhook(c, false)
	if !ok {
		return
	}
	c.JSON(http.StatusOK, s.formatWebhook(webhook, c))
}

func (s *Service) handleUpdateWebhook(c *corehttp.Context) {
	webhook, ok := s.requestWebhook(c, true)
	if !ok {
		return
	}
	body := parseDiscordBody(c.Request)
	patch := corestore.Record{}
	if _, ok := body["name"]; ok {
		patch["name"] = stringValue(body["name"])
	}
	if _, ok := body["avatar"]; ok {
		patch["avatar"] = body["avatar"]
	}
	if _, ok := body["channel_id"]; ok {
		if channel := s.findChannel(stringValue(body["channel_id"])); channel != nil {
			patch["channel_id"] = stringField(channel, "channel_id")
			patch["guild_id"] = stringField(channel, "guild_id")
		}
	}
	updated, _ := s.store.Webhooks.Update(intField(webhook, "id"), patch)
	c.JSON(http.StatusOK, s.formatWebhook(updated, c))
}

func (s *Service) handleDeleteWebhook(c *corehttp.Context) {
	webhook, ok := s.requestWebhook(c, true)
	if !ok {
		return
	}
	s.store.Webhooks.Delete(intField(webhook, "id"))
	c.Writer.WriteHeader(http.StatusNoContent)
}

func (s *Service) handleExecuteWebhook(c *corehttp.Context) {
	webhook := s.findWebhook(c.Param("webhookId"), c.Param("token"))
	if webhook == nil {
		discordError(c, http.StatusNotFound, "Unknown Webhook", 10015)
		return
	}
	channel := s.findChannel(stringField(webhook, "channel_id"))
	if channel == nil {
		discordError(c, http.StatusNotFound, "Unknown Channel", 10003)
		return
	}
	body := parseDiscordBody(c.Request)
	content := stringValue(body["content"])
	embeds := recordSliceValue(body["embeds"])
	attachments := recordSliceValue(body["attachments"])
	if strings.TrimSpace(content) == "" && len(embeds) == 0 && len(attachments) == 0 {
		discordError(c, http.StatusBadRequest, "Cannot send an empty message", 50006)
		return
	}
	message := s.store.Messages.Insert(corestore.Record{
		"message_id":       generateDiscordID(),
		"channel_id":       stringField(channel, "channel_id"),
		"guild_id":         stringField(channel, "guild_id"),
		"author_id":        "",
		"webhook_id":       stringField(webhook, "webhook_id"),
		"content":          content,
		"timestamp":        time.Now().UTC().Format(time.RFC3339Nano),
		"edited_timestamp": nil,
		"tts":              boolValue(body["tts"]),
		"mention_everyone": false,
		"mentions":         []any{},
		"mention_roles":    []any{},
		"attachments":      attachments,
		"embeds":           embeds,
		"reactions":        []map[string]any{},
		"pinned":           false,
		"type":             0,
	})
	s.store.Channels.Update(intField(channel, "id"), corestore.Record{"last_message_id": stringField(message, "message_id")})
	if strings.EqualFold(c.Query("wait"), "true") {
		c.JSON(http.StatusOK, s.formatMessage(message))
		return
	}
	c.Writer.WriteHeader(http.StatusNoContent)
}

func (s *Service) handleGetWebhookMessage(c *corehttp.Context) {
	webhook, message, ok := s.webhookMessage(c)
	_ = webhook
	if !ok {
		return
	}
	c.JSON(http.StatusOK, s.formatMessage(message))
}

func (s *Service) handleUpdateWebhookMessage(c *corehttp.Context) {
	_, message, ok := s.webhookMessage(c)
	if !ok {
		return
	}
	body := parseDiscordBody(c.Request)
	patch := corestore.Record{"edited_timestamp": time.Now().UTC().Format(time.RFC3339Nano)}
	if _, ok := body["content"]; ok {
		patch["content"] = stringValue(body["content"])
	}
	if _, ok := body["embeds"]; ok {
		patch["embeds"] = recordSliceValue(body["embeds"])
	}
	if _, ok := body["attachments"]; ok {
		patch["attachments"] = recordSliceValue(body["attachments"])
	}
	updated, _ := s.store.Messages.Update(intField(message, "id"), patch)
	c.JSON(http.StatusOK, s.formatMessage(updated))
}

func (s *Service) handleDeleteWebhookMessage(c *corehttp.Context) {
	_, message, ok := s.webhookMessage(c)
	if !ok {
		return
	}
	s.store.Messages.Delete(intField(message, "id"))
	c.Writer.WriteHeader(http.StatusNoContent)
}

func (s *Service) handleListApplicationCommands(c *corehttp.Context) {
	if _, ok := s.authenticatedUser(c); !ok {
		return
	}
	commands := s.applicationCommands(c.Param("applicationId"), c.Param("guildId"))
	out := make([]map[string]any, 0, len(commands))
	for _, command := range commands {
		out = append(out, formatApplicationCommand(command))
	}
	sort.SliceStable(out, func(i int, j int) bool {
		return stringValue(out[i]["id"]) < stringValue(out[j]["id"])
	})
	c.JSON(http.StatusOK, out)
}

func (s *Service) handleCreateApplicationCommand(c *corehttp.Context) {
	if _, ok := s.authenticatedUser(c); !ok {
		return
	}
	body := parseDiscordBody(c.Request)
	command := s.insertApplicationCommand(c.Param("applicationId"), c.Param("guildId"), body)
	c.JSON(http.StatusCreated, formatApplicationCommand(command))
}

func (s *Service) handleBulkOverwriteApplicationCommands(c *corehttp.Context) {
	if _, ok := s.authenticatedUser(c); !ok {
		return
	}
	applicationID := c.Param("applicationId")
	guildID := c.Param("guildId")
	for _, command := range s.applicationCommands(applicationID, guildID) {
		s.store.ApplicationCommands.Delete(intField(command, "id"))
	}
	items := parseDiscordBodyArray(c.Request)
	out := []map[string]any{}
	for _, item := range items {
		command := s.insertApplicationCommand(applicationID, guildID, item)
		out = append(out, formatApplicationCommand(command))
	}
	c.JSON(http.StatusOK, out)
}

func (s *Service) handleGetApplicationCommand(c *corehttp.Context) {
	if _, ok := s.authenticatedUser(c); !ok {
		return
	}
	command := s.findApplicationCommand(c.Param("applicationId"), c.Param("guildId"), c.Param("commandId"))
	if command == nil {
		discordError(c, http.StatusNotFound, "Unknown Application Command", 10063)
		return
	}
	c.JSON(http.StatusOK, formatApplicationCommand(command))
}

func (s *Service) handleUpdateApplicationCommand(c *corehttp.Context) {
	if _, ok := s.authenticatedUser(c); !ok {
		return
	}
	command := s.findApplicationCommand(c.Param("applicationId"), c.Param("guildId"), c.Param("commandId"))
	if command == nil {
		discordError(c, http.StatusNotFound, "Unknown Application Command", 10063)
		return
	}
	body := parseDiscordBody(c.Request)
	patch := applicationCommandPatch(body)
	updated, _ := s.store.ApplicationCommands.Update(intField(command, "id"), patch)
	c.JSON(http.StatusOK, formatApplicationCommand(updated))
}

func (s *Service) handleDeleteApplicationCommand(c *corehttp.Context) {
	if _, ok := s.authenticatedUser(c); !ok {
		return
	}
	command := s.findApplicationCommand(c.Param("applicationId"), c.Param("guildId"), c.Param("commandId"))
	if command == nil {
		discordError(c, http.StatusNotFound, "Unknown Application Command", 10063)
		return
	}
	s.store.ApplicationCommands.Delete(intField(command, "id"))
	c.Writer.WriteHeader(http.StatusNoContent)
}

func (s *Service) requestWebhook(c *corehttp.Context, allowAuth bool) (corestore.Record, bool) {
	token := c.Param("token")
	if token == "" {
		if _, ok := s.authenticatedUser(c); !ok {
			return nil, false
		}
	}
	if token != "" && allowAuth && c.Header("Authorization") != "" {
		if _, ok := s.authenticatedUser(c); !ok {
			return nil, false
		}
	}
	webhook := s.findWebhook(c.Param("webhookId"), token)
	if webhook == nil {
		discordError(c, http.StatusNotFound, "Unknown Webhook", 10015)
		return nil, false
	}
	return webhook, true
}

func (s *Service) webhookMessage(c *corehttp.Context) (corestore.Record, corestore.Record, bool) {
	webhook := s.findWebhook(c.Param("webhookId"), c.Param("token"))
	if webhook == nil {
		discordError(c, http.StatusNotFound, "Unknown Webhook", 10015)
		return nil, nil, false
	}
	message := s.findMessage(stringField(webhook, "channel_id"), c.Param("messageId"))
	if message == nil || stringField(message, "webhook_id") != stringField(webhook, "webhook_id") {
		discordError(c, http.StatusNotFound, "Unknown Message", 10008)
		return nil, nil, false
	}
	return webhook, message, true
}

func (s *Service) findWebhook(webhookID string, token string) corestore.Record {
	for _, webhook := range s.store.Webhooks.FindBy("webhook_id", webhookID) {
		if token == "" || stringField(webhook, "token") == token {
			return webhook
		}
	}
	return nil
}

func (s *Service) formatWebhook(webhook corestore.Record, c *corehttp.Context) map[string]any {
	url := "http://" + c.Request.Host + "/api/webhooks/" + stringField(webhook, "webhook_id") + "/" + stringField(webhook, "token")
	if c.Request.TLS != nil {
		url = "https://" + c.Request.Host + "/api/webhooks/" + stringField(webhook, "webhook_id") + "/" + stringField(webhook, "token")
	}
	out := map[string]any{
		"id":             stringField(webhook, "webhook_id"),
		"type":           intField(webhook, "type"),
		"guild_id":       stringField(webhook, "guild_id"),
		"channel_id":     stringField(webhook, "channel_id"),
		"name":           stringField(webhook, "name"),
		"avatar":         webhook["avatar"],
		"application_id": webhook["application_id"],
		"token":          stringField(webhook, "token"),
		"url":            url,
	}
	if user := s.findUser(stringField(webhook, "user_id")); user != nil {
		out["user"] = formatUser(user)
	}
	return out
}

func (s *Service) insertApplicationCommand(applicationID string, guildID string, body map[string]any) corestore.Record {
	name := strings.TrimSpace(stringValue(body["name"]))
	if name == "" {
		name = "command"
	}
	description := stringValue(body["description"])
	if description == "" {
		description = name + " command"
	}
	return s.store.ApplicationCommands.Insert(corestore.Record{
		"command_id":                 generateDiscordID(),
		"application_id":             applicationID,
		"guild_id":                   guildID,
		"name":                       name,
		"description":                description,
		"type":                       intValueWithDefault(body["type"], 1),
		"options":                    recordSliceValue(body["options"]),
		"default_member_permissions": body["default_member_permissions"],
		"dm_permission":              body["dm_permission"],
		"version":                    generateDiscordID(),
	})
}

func (s *Service) applicationCommands(applicationID string, guildID string) []corestore.Record {
	commands := []corestore.Record{}
	for _, command := range s.store.ApplicationCommands.FindBy("application_id", applicationID) {
		if stringField(command, "guild_id") == guildID {
			commands = append(commands, command)
		}
	}
	return commands
}

func (s *Service) findApplicationCommand(applicationID string, guildID string, commandID string) corestore.Record {
	for _, command := range s.store.ApplicationCommands.FindBy("command_id", commandID) {
		if stringField(command, "application_id") == applicationID && stringField(command, "guild_id") == guildID {
			return command
		}
	}
	return nil
}

func applicationCommandPatch(body map[string]any) corestore.Record {
	patch := corestore.Record{"version": generateDiscordID()}
	for _, key := range []string{"name", "description"} {
		if _, ok := body[key]; ok {
			patch[key] = stringValue(body[key])
		}
	}
	if _, ok := body["type"]; ok {
		patch["type"] = intValue(body["type"])
	}
	for _, key := range []string{"options"} {
		if _, ok := body[key]; ok {
			patch[key] = recordSliceValue(body[key])
		}
	}
	for _, key := range []string{"default_member_permissions", "dm_permission"} {
		if _, ok := body[key]; ok {
			patch[key] = body[key]
		}
	}
	return patch
}

func formatApplicationCommand(command corestore.Record) map[string]any {
	out := map[string]any{
		"id":                         stringField(command, "command_id"),
		"application_id":             stringField(command, "application_id"),
		"name":                       stringField(command, "name"),
		"description":                stringField(command, "description"),
		"type":                       intField(command, "type"),
		"options":                    recordSliceValue(command["options"]),
		"default_member_permissions": command["default_member_permissions"],
		"dm_permission":              command["dm_permission"],
		"version":                    stringField(command, "version"),
	}
	if guildID := stringField(command, "guild_id"); guildID != "" {
		out["guild_id"] = guildID
	}
	return out
}

func intValueWithDefault(value any, fallback int) int {
	if value == nil {
		return fallback
	}
	return intValue(value)
}
