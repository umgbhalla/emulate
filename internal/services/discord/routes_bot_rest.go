package discord

import (
	"net/http"
	"sort"
	"strings"
	"time"

	corehttp "github.com/vercel-labs/emulate/internal/core/http"
	corestore "github.com/vercel-labs/emulate/internal/core/store"
)

type roleInput struct {
	RoleID      string
	GuildID     string
	Name        string
	Color       int
	Hoist       bool
	Position    int
	Permissions string
	Managed     bool
	Mentionable bool
}

func roleRecord(input roleInput) corestore.Record {
	permissions := input.Permissions
	if permissions == "" {
		permissions = "0"
	}
	return corestore.Record{
		"role_id":     input.RoleID,
		"guild_id":    input.GuildID,
		"name":        firstNonEmpty(input.Name, "new role"),
		"color":       input.Color,
		"hoist":       input.Hoist,
		"position":    input.Position,
		"permissions": permissions,
		"managed":     input.Managed,
		"mentionable": input.Mentionable,
	}
}

func (s *Service) handleGetGateway(c *corehttp.Context) {
	c.JSON(http.StatusOK, map[string]any{"url": s.gatewayURL(c)})
}

func (s *Service) handleGetGatewayBot(c *corehttp.Context) {
	if _, ok := s.authenticatedUser(c); !ok {
		return
	}
	c.JSON(http.StatusOK, map[string]any{
		"url":    s.gatewayURL(c),
		"shards": 1,
		"session_start_limit": map[string]any{
			"total":           1000,
			"remaining":       1000,
			"reset_after":     14400000,
			"max_concurrency": 1,
		},
	})
}

func (s *Service) gatewayURL(c *corehttp.Context) string {
	scheme := "ws"
	if c.Request.TLS != nil {
		scheme = "wss"
	}
	return scheme + "://" + c.Request.Host + "/gateway"
}

func (s *Service) handleGetMessage(c *corehttp.Context) {
	if _, ok := s.authenticatedUser(c); !ok {
		return
	}
	_, message, ok := s.channelMessage(c)
	if !ok {
		return
	}
	c.JSON(http.StatusOK, s.formatMessage(message))
}

func (s *Service) handleCreateGuild(c *corehttp.Context) {
	user, ok := s.authenticatedUser(c)
	if !ok {
		return
	}
	body := parseDiscordBody(c.Request)
	name := strings.TrimSpace(stringValue(body["name"]))
	if name == "" {
		discordError(c, http.StatusBadRequest, "Invalid Form Body", 50035)
		return
	}
	guildID := generateDiscordID()
	guild := s.store.Guilds.Insert(corestore.Record{
		"guild_id": guildID,
		"name":     name,
		"icon":     body["icon"],
		"owner_id": firstNonEmpty(stringValue(body["owner_id"]), stringField(user, "user_id")),
	})
	s.store.Roles.Insert(roleRecord(roleInput{
		RoleID:      guildID,
		GuildID:     guildID,
		Name:        "@everyone",
		Permissions: "8",
	}))
	c.JSON(http.StatusCreated, formatGuild(guild))
}

func (s *Service) handleUpdateGuild(c *corehttp.Context) {
	if _, ok := s.authenticatedUser(c); !ok {
		return
	}
	guild := s.findGuild(c.Param("guildId"))
	if guild == nil {
		discordError(c, http.StatusNotFound, "Unknown Guild", 10004)
		return
	}
	body := parseDiscordBody(c.Request)
	patch := corestore.Record{}
	if _, ok := body["name"]; ok {
		patch["name"] = stringValue(body["name"])
	}
	if _, ok := body["icon"]; ok {
		patch["icon"] = body["icon"]
	}
	if _, ok := body["owner_id"]; ok {
		patch["owner_id"] = stringValue(body["owner_id"])
	}
	updated, _ := s.store.Guilds.Update(intField(guild, "id"), patch)
	c.JSON(http.StatusOK, formatGuild(updated))
}

func (s *Service) handleDeleteGuild(c *corehttp.Context) {
	if _, ok := s.authenticatedUser(c); !ok {
		return
	}
	guild := s.findGuild(c.Param("guildId"))
	if guild == nil {
		discordError(c, http.StatusNotFound, "Unknown Guild", 10004)
		return
	}
	guildID := stringField(guild, "guild_id")
	for _, message := range s.store.Messages.FindBy("guild_id", guildID) {
		s.store.Messages.Delete(intField(message, "id"))
	}
	for _, channel := range s.store.Channels.FindBy("guild_id", guildID) {
		s.store.Channels.Delete(intField(channel, "id"))
	}
	for _, role := range s.store.Roles.FindBy("guild_id", guildID) {
		s.store.Roles.Delete(intField(role, "id"))
	}
	for _, memberRole := range s.store.MemberRoles.FindBy("guild_id", guildID) {
		s.store.MemberRoles.Delete(intField(memberRole, "id"))
	}
	for _, webhook := range s.store.Webhooks.FindBy("guild_id", guildID) {
		s.store.Webhooks.Delete(intField(webhook, "id"))
	}
	for _, command := range s.store.ApplicationCommands.FindBy("guild_id", guildID) {
		s.store.ApplicationCommands.Delete(intField(command, "id"))
	}
	s.store.Guilds.Delete(intField(guild, "id"))
	c.Writer.WriteHeader(http.StatusNoContent)
}

func (s *Service) handleTyping(c *corehttp.Context) {
	if _, ok := s.authenticatedUser(c); !ok {
		return
	}
	if s.findChannel(c.Param("channelId")) == nil {
		discordError(c, http.StatusNotFound, "Unknown Channel", 10003)
		return
	}
	c.Writer.WriteHeader(http.StatusNoContent)
}

func (s *Service) handleListPins(c *corehttp.Context) {
	if _, ok := s.authenticatedUser(c); !ok {
		return
	}
	channel := s.findChannel(c.Param("channelId"))
	if channel == nil {
		discordError(c, http.StatusNotFound, "Unknown Channel", 10003)
		return
	}
	messages := []map[string]any{}
	for _, message := range s.store.Messages.FindBy("channel_id", stringField(channel, "channel_id")) {
		if boolField(message, "pinned") {
			messages = append(messages, s.formatMessage(message))
		}
	}
	sort.SliceStable(messages, func(i int, j int) bool {
		return stringValue(messages[i]["id"]) > stringValue(messages[j]["id"])
	})
	c.JSON(http.StatusOK, messages)
}

func (s *Service) handlePinMessage(c *corehttp.Context) {
	if _, ok := s.authenticatedUser(c); !ok {
		return
	}
	_, message, ok := s.channelMessage(c)
	if !ok {
		return
	}
	s.store.Messages.Update(intField(message, "id"), corestore.Record{"pinned": true})
	c.Writer.WriteHeader(http.StatusNoContent)
}

func (s *Service) handleUnpinMessage(c *corehttp.Context) {
	if _, ok := s.authenticatedUser(c); !ok {
		return
	}
	_, message, ok := s.channelMessage(c)
	if !ok {
		return
	}
	s.store.Messages.Update(intField(message, "id"), corestore.Record{"pinned": false})
	c.Writer.WriteHeader(http.StatusNoContent)
}

func (s *Service) handleBulkDeleteMessages(c *corehttp.Context) {
	if _, ok := s.authenticatedUser(c); !ok {
		return
	}
	channel := s.findChannel(c.Param("channelId"))
	if channel == nil {
		discordError(c, http.StatusNotFound, "Unknown Channel", 10003)
		return
	}
	body := parseDiscordBody(c.Request)
	ids := stringSliceValue(body["messages"])
	for _, messageID := range ids {
		if message := s.findMessage(stringField(channel, "channel_id"), messageID); message != nil {
			s.store.Messages.Delete(intField(message, "id"))
		}
	}
	c.Writer.WriteHeader(http.StatusNoContent)
}

func (s *Service) handleAddOwnReaction(c *corehttp.Context) {
	user, ok := s.authenticatedUser(c)
	if !ok {
		return
	}
	_, message, ok := s.channelMessage(c)
	if !ok {
		return
	}
	s.addReaction(message, c.Param("emoji"), stringField(user, "user_id"))
	c.Writer.WriteHeader(http.StatusNoContent)
}

func (s *Service) handleRemoveOwnReaction(c *corehttp.Context) {
	user, ok := s.authenticatedUser(c)
	if !ok {
		return
	}
	_, message, ok := s.channelMessage(c)
	if !ok {
		return
	}
	s.removeReactionUser(message, c.Param("emoji"), stringField(user, "user_id"))
	c.Writer.WriteHeader(http.StatusNoContent)
}

func (s *Service) handleListReactionUsers(c *corehttp.Context) {
	if _, ok := s.authenticatedUser(c); !ok {
		return
	}
	_, message, ok := s.channelMessage(c)
	if !ok {
		return
	}
	limit := normalizeLimit(c.Query("limit"), 25, 100)
	users := []map[string]any{}
	for _, userID := range s.reactionUsers(message, c.Param("emoji")) {
		if user := s.findUser(userID); user != nil {
			users = append(users, formatUser(user))
		}
	}
	if len(users) > limit {
		users = users[:limit]
	}
	c.JSON(http.StatusOK, users)
}

func (s *Service) handleRemoveUserReaction(c *corehttp.Context) {
	if _, ok := s.authenticatedUser(c); !ok {
		return
	}
	_, message, ok := s.channelMessage(c)
	if !ok {
		return
	}
	s.removeReactionUser(message, c.Param("emoji"), c.Param("userId"))
	c.Writer.WriteHeader(http.StatusNoContent)
}

func (s *Service) handleClearReaction(c *corehttp.Context) {
	if _, ok := s.authenticatedUser(c); !ok {
		return
	}
	_, message, ok := s.channelMessage(c)
	if !ok {
		return
	}
	s.clearReaction(message, c.Param("emoji"))
	c.Writer.WriteHeader(http.StatusNoContent)
}

func (s *Service) handleClearReactions(c *corehttp.Context) {
	if _, ok := s.authenticatedUser(c); !ok {
		return
	}
	_, message, ok := s.channelMessage(c)
	if !ok {
		return
	}
	s.store.Messages.Update(intField(message, "id"), corestore.Record{"reactions": []map[string]any{}})
	c.Writer.WriteHeader(http.StatusNoContent)
}

func (s *Service) handleCreateGuildChannel(c *corehttp.Context) {
	if _, ok := s.authenticatedUser(c); !ok {
		return
	}
	guild := s.findGuild(c.Param("guildId"))
	if guild == nil {
		discordError(c, http.StatusNotFound, "Unknown Guild", 10004)
		return
	}
	body := parseDiscordBody(c.Request)
	name := strings.TrimSpace(stringValue(body["name"]))
	if name == "" {
		discordError(c, http.StatusBadRequest, "Invalid Form Body", 50035)
		return
	}
	channel := s.store.Channels.Insert(channelRecord(channelInput{
		ChannelID: generateDiscordID(),
		GuildID:   stringField(guild, "guild_id"),
		Name:      name,
		Topic:     stringValue(body["topic"]),
		Type:      intValue(body["type"]),
		Position:  s.store.Channels.Count(),
	}))
	c.JSON(http.StatusCreated, formatChannel(channel))
}

func (s *Service) handleUpdateChannel(c *corehttp.Context) {
	if _, ok := s.authenticatedUser(c); !ok {
		return
	}
	channel := s.findChannel(c.Param("channelId"))
	if channel == nil {
		discordError(c, http.StatusNotFound, "Unknown Channel", 10003)
		return
	}
	body := parseDiscordBody(c.Request)
	patch := corestore.Record{}
	if _, ok := body["name"]; ok {
		patch["name"] = stringValue(body["name"])
	}
	if _, ok := body["topic"]; ok {
		patch["topic"] = stringValue(body["topic"])
	}
	if _, ok := body["type"]; ok {
		patch["type"] = intValue(body["type"])
	}
	if _, ok := body["position"]; ok {
		patch["position"] = intValue(body["position"])
	}
	if _, ok := body["nsfw"]; ok {
		patch["nsfw"] = boolValue(body["nsfw"])
	}
	updated, _ := s.store.Channels.Update(intField(channel, "id"), patch)
	c.JSON(http.StatusOK, formatChannel(updated))
}

func (s *Service) handleDeleteChannel(c *corehttp.Context) {
	if _, ok := s.authenticatedUser(c); !ok {
		return
	}
	channel := s.findChannel(c.Param("channelId"))
	if channel == nil {
		discordError(c, http.StatusNotFound, "Unknown Channel", 10003)
		return
	}
	for _, message := range s.store.Messages.FindBy("channel_id", stringField(channel, "channel_id")) {
		s.store.Messages.Delete(intField(message, "id"))
	}
	for _, webhook := range s.store.Webhooks.FindBy("channel_id", stringField(channel, "channel_id")) {
		s.store.Webhooks.Delete(intField(webhook, "id"))
	}
	s.store.Channels.Delete(intField(channel, "id"))
	c.JSON(http.StatusOK, formatChannel(channel))
}

func (s *Service) handleListGuildRoles(c *corehttp.Context) {
	if _, ok := s.authenticatedUser(c); !ok {
		return
	}
	guild := s.findGuild(c.Param("guildId"))
	if guild == nil {
		discordError(c, http.StatusNotFound, "Unknown Guild", 10004)
		return
	}
	roles := []map[string]any{}
	for _, role := range s.store.Roles.FindBy("guild_id", stringField(guild, "guild_id")) {
		roles = append(roles, formatRole(role))
	}
	sort.SliceStable(roles, func(i int, j int) bool {
		return intValue(roles[i]["position"]) < intValue(roles[j]["position"])
	})
	c.JSON(http.StatusOK, roles)
}

func (s *Service) handleCreateGuildRole(c *corehttp.Context) {
	if _, ok := s.authenticatedUser(c); !ok {
		return
	}
	guild := s.findGuild(c.Param("guildId"))
	if guild == nil {
		discordError(c, http.StatusNotFound, "Unknown Guild", 10004)
		return
	}
	body := parseDiscordBody(c.Request)
	role := s.store.Roles.Insert(roleRecord(roleInput{
		RoleID:      generateDiscordID(),
		GuildID:     stringField(guild, "guild_id"),
		Name:        firstNonEmpty(stringValue(body["name"]), "new role"),
		Color:       intValue(body["color"]),
		Hoist:       boolValue(body["hoist"]),
		Position:    s.store.Roles.Count(),
		Permissions: stringValue(body["permissions"]),
		Mentionable: boolValue(body["mentionable"]),
	}))
	c.JSON(http.StatusOK, formatRole(role))
}

func (s *Service) handleUpdateGuildRole(c *corehttp.Context) {
	if _, ok := s.authenticatedUser(c); !ok {
		return
	}
	role := s.findRole(c.Param("guildId"), c.Param("roleId"))
	if role == nil {
		discordError(c, http.StatusNotFound, "Unknown Role", 10011)
		return
	}
	body := parseDiscordBody(c.Request)
	patch := corestore.Record{}
	for _, key := range []string{"name", "permissions"} {
		if _, ok := body[key]; ok {
			patch[key] = stringValue(body[key])
		}
	}
	for _, key := range []string{"color", "position"} {
		if _, ok := body[key]; ok {
			patch[key] = intValue(body[key])
		}
	}
	for _, key := range []string{"hoist", "mentionable"} {
		if _, ok := body[key]; ok {
			patch[key] = boolValue(body[key])
		}
	}
	updated, _ := s.store.Roles.Update(intField(role, "id"), patch)
	c.JSON(http.StatusOK, formatRole(updated))
}

func (s *Service) handleDeleteGuildRole(c *corehttp.Context) {
	if _, ok := s.authenticatedUser(c); !ok {
		return
	}
	role := s.findRole(c.Param("guildId"), c.Param("roleId"))
	if role == nil {
		discordError(c, http.StatusNotFound, "Unknown Role", 10011)
		return
	}
	for _, memberRole := range s.store.MemberRoles.FindBy("role_id", stringField(role, "role_id")) {
		s.store.MemberRoles.Delete(intField(memberRole, "id"))
	}
	s.store.Roles.Delete(intField(role, "id"))
	c.Writer.WriteHeader(http.StatusNoContent)
}

func (s *Service) handleAddGuildMemberRole(c *corehttp.Context) {
	if _, ok := s.authenticatedUser(c); !ok {
		return
	}
	if s.findGuild(c.Param("guildId")) == nil || s.findUser(c.Param("userId")) == nil || s.findRole(c.Param("guildId"), c.Param("roleId")) == nil {
		discordError(c, http.StatusNotFound, "Unknown Member or Role", 10007)
		return
	}
	s.addMemberRole(c.Param("guildId"), c.Param("userId"), c.Param("roleId"))
	c.Writer.WriteHeader(http.StatusNoContent)
}

func (s *Service) handleRemoveGuildMemberRole(c *corehttp.Context) {
	if _, ok := s.authenticatedUser(c); !ok {
		return
	}
	s.removeMemberRole(c.Param("guildId"), c.Param("userId"), c.Param("roleId"))
	c.Writer.WriteHeader(http.StatusNoContent)
}

func (s *Service) handleUpdateGuildMember(c *corehttp.Context) {
	if _, ok := s.authenticatedUser(c); !ok {
		return
	}
	if s.findGuild(c.Param("guildId")) == nil {
		discordError(c, http.StatusNotFound, "Unknown Guild", 10004)
		return
	}
	user := s.findUser(c.Param("userId"))
	if user == nil {
		discordError(c, http.StatusNotFound, "Unknown Member", 10007)
		return
	}
	body := parseDiscordBody(c.Request)
	if _, ok := body["roles"]; ok {
		for _, memberRole := range s.store.MemberRoles.FindBy("user_id", stringField(user, "user_id")) {
			if stringField(memberRole, "guild_id") == c.Param("guildId") {
				s.store.MemberRoles.Delete(intField(memberRole, "id"))
			}
		}
		for _, roleID := range stringSliceValue(body["roles"]) {
			s.addMemberRole(c.Param("guildId"), stringField(user, "user_id"), roleID)
		}
	}
	c.JSON(http.StatusOK, s.formatMemberForGuild(c.Param("guildId"), user))
}

func (s *Service) channelMessage(c *corehttp.Context) (corestore.Record, corestore.Record, bool) {
	channel := s.findChannel(c.Param("channelId"))
	if channel == nil {
		discordError(c, http.StatusNotFound, "Unknown Channel", 10003)
		return nil, nil, false
	}
	message := s.findMessage(stringField(channel, "channel_id"), c.Param("messageId"))
	if message == nil {
		discordError(c, http.StatusNotFound, "Unknown Message", 10008)
		return nil, nil, false
	}
	return channel, message, true
}

func (s *Service) addReaction(message corestore.Record, emoji string, userID string) {
	s.store.Messages.UpdateFunc(intField(message, "id"), func(current corestore.Record) (corestore.Record, bool) {
		reactions := recordSliceValue(current["reactions"])
		for index, reaction := range reactions {
			if stringValue(reaction["name"]) != emoji {
				continue
			}
			users := stringSliceValue(reaction["users"])
			if containsString(users, userID) {
				return corestore.Record{"reactions": reactions}, true
			}
			users = append(users, userID)
			reactions[index]["users"] = users
			reactions[index]["count"] = len(users)
			return corestore.Record{"reactions": reactions}, true
		}
		reactions = append(reactions, map[string]any{"name": emoji, "users": []string{userID}, "count": 1})
		return corestore.Record{"reactions": reactions}, true
	})
}

func (s *Service) removeReactionUser(message corestore.Record, emoji string, userID string) {
	s.store.Messages.UpdateFunc(intField(message, "id"), func(current corestore.Record) (corestore.Record, bool) {
		reactions := recordSliceValue(current["reactions"])
		next := []map[string]any{}
		for _, reaction := range reactions {
			if stringValue(reaction["name"]) != emoji {
				next = append(next, reaction)
				continue
			}
			users := removeString(stringSliceValue(reaction["users"]), userID)
			if len(users) > 0 {
				reaction["users"] = users
				reaction["count"] = len(users)
				next = append(next, reaction)
			}
		}
		return corestore.Record{"reactions": next}, true
	})
}

func (s *Service) clearReaction(message corestore.Record, emoji string) {
	s.store.Messages.UpdateFunc(intField(message, "id"), func(current corestore.Record) (corestore.Record, bool) {
		reactions := recordSliceValue(current["reactions"])
		next := []map[string]any{}
		for _, reaction := range reactions {
			if stringValue(reaction["name"]) != emoji {
				next = append(next, reaction)
			}
		}
		return corestore.Record{"reactions": next}, true
	})
}

func (s *Service) reactionUsers(message corestore.Record, emoji string) []string {
	for _, reaction := range recordSliceValue(message["reactions"]) {
		if stringValue(reaction["name"]) == emoji {
			return stringSliceValue(reaction["users"])
		}
	}
	return nil
}

func (s *Service) findRole(guildID string, roleID string) corestore.Record {
	for _, role := range s.store.Roles.FindBy("role_id", roleID) {
		if stringField(role, "guild_id") == guildID {
			return role
		}
	}
	return nil
}

func (s *Service) addMemberRole(guildID string, userID string, roleID string) {
	for _, memberRole := range s.store.MemberRoles.FindBy("user_id", userID) {
		if stringField(memberRole, "guild_id") == guildID && stringField(memberRole, "role_id") == roleID {
			return
		}
	}
	s.store.MemberRoles.Insert(corestore.Record{"guild_id": guildID, "user_id": userID, "role_id": roleID})
}

func (s *Service) removeMemberRole(guildID string, userID string, roleID string) {
	for _, memberRole := range s.store.MemberRoles.FindBy("user_id", userID) {
		if stringField(memberRole, "guild_id") == guildID && stringField(memberRole, "role_id") == roleID {
			s.store.MemberRoles.Delete(intField(memberRole, "id"))
		}
	}
}

func (s *Service) memberRoleIDs(guildID string, userID string) []string {
	roles := []string{}
	for _, memberRole := range s.store.MemberRoles.FindBy("user_id", userID) {
		if stringField(memberRole, "guild_id") == guildID {
			roles = append(roles, stringField(memberRole, "role_id"))
		}
	}
	sort.Strings(roles)
	return roles
}

func formatRole(role corestore.Record) map[string]any {
	return map[string]any{
		"id":            stringField(role, "role_id"),
		"name":          stringField(role, "name"),
		"color":         intField(role, "color"),
		"hoist":         boolField(role, "hoist"),
		"position":      intField(role, "position"),
		"permissions":   stringField(role, "permissions"),
		"managed":       boolField(role, "managed"),
		"mentionable":   boolField(role, "mentionable"),
		"tags":          map[string]any{},
		"unicode_emoji": nil,
	}
}

func boolValue(value any) bool {
	switch v := value.(type) {
	case bool:
		return v
	case string:
		return v == "true"
	default:
		return false
	}
}

func containsString(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func removeString(values []string, target string) []string {
	next := values[:0]
	for _, value := range values {
		if value != target {
			next = append(next, value)
		}
	}
	return next
}

func nowTimestamp() string {
	return time.Now().UTC().Format(time.RFC3339Nano)
}
