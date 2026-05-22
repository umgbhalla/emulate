package discord

import (
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	corehttp "github.com/vercel-labs/emulate/internal/core/http"
	corestore "github.com/vercel-labs/emulate/internal/core/store"
	"github.com/vercel-labs/emulate/internal/core/ui"
)

const serviceLabel = "Discord"

type Options struct {
	Store         *corestore.Store
	Seed          *SeedConfig
	RootInspector bool
}

type SeedConfig struct {
	Port        int              `json:"port,omitempty"`
	Application *ApplicationSeed `json:"application"`
	Guild       *GuildSeed       `json:"guild"`
	Users       []UserSeed       `json:"users"`
	Channels    []ChannelSeed    `json:"channels"`
	Bot         *BotSeed         `json:"bot"`
}

type ApplicationSeed struct {
	ID           string   `json:"id"`
	ClientID     string   `json:"client_id"`
	ClientSecret string   `json:"client_secret"`
	Name         string   `json:"name"`
	RedirectURIs []string `json:"redirect_uris"`
	PublicKey    string   `json:"public_key"`
}

type GuildSeed struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type UserSeed struct {
	ID         string `json:"id"`
	Username   string `json:"username"`
	GlobalName string `json:"global_name"`
	Email      string `json:"email"`
	Bot        bool   `json:"bot"`
}

type ChannelSeed struct {
	ID      string `json:"id"`
	GuildID string `json:"guild_id"`
	Name    string `json:"name"`
	Topic   string `json:"topic"`
	Type    int    `json:"type"`
}

type BotSeed struct {
	ID       string `json:"id"`
	Username string `json:"username"`
	Token    string `json:"token"`
}

type Service struct {
	store         Store
	rootInspector bool
}

func Register(router *corehttp.Router, options Options) {
	service := New(options)
	service.RegisterRoutes(router)
}

func New(options Options) *Service {
	runtimeStore := options.Store
	if runtimeStore == nil {
		runtimeStore = corestore.New()
	}
	service := &Service{
		store:         NewStore(runtimeStore),
		rootInspector: options.RootInspector,
	}
	service.SeedDefaults()
	if options.Seed != nil {
		service.SeedFromConfig(*options.Seed)
	}
	return service
}

func SeedFromConfig(runtimeStore *corestore.Store, config SeedConfig) {
	New(Options{Store: runtimeStore, Seed: &config})
}

func (s *Service) RegisterRoutes(router *corehttp.Router) {
	for _, prefix := range []string{"/api/v10", "/api/v9", "/api"} {
		s.registerAPIRoutes(router, prefix)
	}
	s.registerOAuthRoutes(router)
	router.Get("/discord", s.handleInspector)
	if s.rootInspector {
		router.Get("/", s.handleInspector)
	}
}

func (s *Service) registerAPIRoutes(router *corehttp.Router, prefix string) {
	router.Get(prefix+"/gateway", s.handleGetGateway)
	router.Get(prefix+"/gateway/bot", s.handleGetGatewayBot)
	router.Get(prefix+"/users/@me", s.handleCurrentUser)
	router.Get(prefix+"/users/@me/guilds", s.handleCurrentUserGuilds)
	router.Get(prefix+"/oauth2/applications/@me", s.handleCurrentApplication)
	router.Post(prefix+"/guilds", s.handleCreateGuild)
	router.Get(prefix+"/guilds/:guildId", s.handleGetGuild)
	router.Patch(prefix+"/guilds/:guildId", s.handleUpdateGuild)
	router.Delete(prefix+"/guilds/:guildId", s.handleDeleteGuild)
	router.Get(prefix+"/guilds/:guildId/channels", s.handleGuildChannels)
	router.Post(prefix+"/guilds/:guildId/channels", s.handleCreateGuildChannel)
	router.Get(prefix+"/guilds/:guildId/members", s.handleGuildMembers)
	router.Get(prefix+"/guilds/:guildId/members/:userId", s.handleGuildMember)
	router.Patch(prefix+"/guilds/:guildId/members/:userId", s.handleUpdateGuildMember)
	router.Get(prefix+"/guilds/:guildId/roles", s.handleListGuildRoles)
	router.Post(prefix+"/guilds/:guildId/roles", s.handleCreateGuildRole)
	router.Patch(prefix+"/guilds/:guildId/roles/:roleId", s.handleUpdateGuildRole)
	router.Delete(prefix+"/guilds/:guildId/roles/:roleId", s.handleDeleteGuildRole)
	router.Put(prefix+"/guilds/:guildId/members/:userId/roles/:roleId", s.handleAddGuildMemberRole)
	router.Delete(prefix+"/guilds/:guildId/members/:userId/roles/:roleId", s.handleRemoveGuildMemberRole)
	router.Get(prefix+"/channels/:channelId", s.handleGetChannel)
	router.Patch(prefix+"/channels/:channelId", s.handleUpdateChannel)
	router.Delete(prefix+"/channels/:channelId", s.handleDeleteChannel)
	router.Get(prefix+"/channels/:channelId/messages", s.handleListMessages)
	router.Post(prefix+"/channels/:channelId/messages", s.handleCreateMessage)
	router.Get(prefix+"/channels/:channelId/messages/pins", s.handleListPins)
	router.Put(prefix+"/channels/:channelId/messages/pins/:messageId", s.handlePinMessage)
	router.Delete(prefix+"/channels/:channelId/messages/pins/:messageId", s.handleUnpinMessage)
	router.Get(prefix+"/channels/:channelId/messages/:messageId", s.handleGetMessage)
	router.Patch(prefix+"/channels/:channelId/messages/:messageId", s.handleUpdateMessage)
	router.Delete(prefix+"/channels/:channelId/messages/:messageId", s.handleDeleteMessage)
	router.Post(prefix+"/channels/:channelId/messages/bulk-delete", s.handleBulkDeleteMessages)
	router.Post(prefix+"/channels/:channelId/typing", s.handleTyping)
	router.Get(prefix+"/channels/:channelId/pins", s.handleListPins)
	router.Put(prefix+"/channels/:channelId/pins/:messageId", s.handlePinMessage)
	router.Delete(prefix+"/channels/:channelId/pins/:messageId", s.handleUnpinMessage)
	router.Put(prefix+"/channels/:channelId/messages/:messageId/reactions/:emoji/@me", s.handleAddOwnReaction)
	router.Delete(prefix+"/channels/:channelId/messages/:messageId/reactions/:emoji/@me", s.handleRemoveOwnReaction)
	router.Get(prefix+"/channels/:channelId/messages/:messageId/reactions/:emoji", s.handleListReactionUsers)
	router.Delete(prefix+"/channels/:channelId/messages/:messageId/reactions/:emoji/:userId", s.handleRemoveUserReaction)
	router.Delete(prefix+"/channels/:channelId/messages/:messageId/reactions/:emoji", s.handleClearReaction)
	router.Delete(prefix+"/channels/:channelId/messages/:messageId/reactions", s.handleClearReactions)
	router.Get(prefix+"/channels/:channelId/webhooks", s.handleListChannelWebhooks)
	router.Post(prefix+"/channels/:channelId/webhooks", s.handleCreateChannelWebhook)
	router.Get(prefix+"/webhooks/:webhookId", s.handleGetWebhook)
	router.Get(prefix+"/webhooks/:webhookId/:token", s.handleGetWebhook)
	router.Patch(prefix+"/webhooks/:webhookId", s.handleUpdateWebhook)
	router.Patch(prefix+"/webhooks/:webhookId/:token", s.handleUpdateWebhook)
	router.Delete(prefix+"/webhooks/:webhookId", s.handleDeleteWebhook)
	router.Delete(prefix+"/webhooks/:webhookId/:token", s.handleDeleteWebhook)
	router.Post(prefix+"/webhooks/:webhookId/:token", s.handleExecuteWebhook)
	router.Get(prefix+"/webhooks/:webhookId/:token/messages/:messageId", s.handleGetWebhookMessage)
	router.Patch(prefix+"/webhooks/:webhookId/:token/messages/:messageId", s.handleUpdateWebhookMessage)
	router.Delete(prefix+"/webhooks/:webhookId/:token/messages/:messageId", s.handleDeleteWebhookMessage)
	router.Get(prefix+"/applications/:applicationId/commands", s.handleListApplicationCommands)
	router.Post(prefix+"/applications/:applicationId/commands", s.handleCreateApplicationCommand)
	router.Put(prefix+"/applications/:applicationId/commands", s.handleBulkOverwriteApplicationCommands)
	router.Get(prefix+"/applications/:applicationId/commands/:commandId", s.handleGetApplicationCommand)
	router.Patch(prefix+"/applications/:applicationId/commands/:commandId", s.handleUpdateApplicationCommand)
	router.Delete(prefix+"/applications/:applicationId/commands/:commandId", s.handleDeleteApplicationCommand)
	router.Get(prefix+"/applications/:applicationId/guilds/:guildId/commands", s.handleListApplicationCommands)
	router.Post(prefix+"/applications/:applicationId/guilds/:guildId/commands", s.handleCreateApplicationCommand)
	router.Put(prefix+"/applications/:applicationId/guilds/:guildId/commands", s.handleBulkOverwriteApplicationCommands)
	router.Get(prefix+"/applications/:applicationId/guilds/:guildId/commands/:commandId", s.handleGetApplicationCommand)
	router.Patch(prefix+"/applications/:applicationId/guilds/:guildId/commands/:commandId", s.handleUpdateApplicationCommand)
	router.Delete(prefix+"/applications/:applicationId/guilds/:guildId/commands/:commandId", s.handleDeleteApplicationCommand)
}

func (s *Service) SeedDefaults() {
	if firstRecord(s.store.Guilds.FindBy("guild_id", "100000000000000001")) != nil {
		return
	}
	guildID := "100000000000000001"
	botID := "200000000000000001"
	userID := "200000000000000002"
	s.store.Applications.Insert(corestore.Record{
		"application_id": "900000000000000001",
		"client_id":      "discord-client-id",
		"client_secret":  "discord-client-secret",
		"name":           "Emulate Discord App",
		"bot_id":         botID,
		"redirect_uris": []string{
			"http://localhost:3000/api/auth/callback/discord",
			"http://localhost:3000/callback",
		},
		"public_key": "",
	})
	s.store.Guilds.Insert(corestore.Record{
		"guild_id": guildID,
		"name":     "Emulate",
		"icon":     nil,
		"owner_id": userID,
	})
	s.store.Users.Insert(userRecord(userInput{
		UserID:     botID,
		Username:   "emulate-bot",
		GlobalName: "Emulate Bot",
		Bot:        true,
	}))
	s.store.Users.Insert(userRecord(userInput{
		UserID:     userID,
		Username:   "developer",
		GlobalName: "Developer",
		Email:      "dev@example.com",
	}))
	s.store.Channels.Insert(channelRecord(channelInput{
		ChannelID: "300000000000000001",
		GuildID:   guildID,
		Name:      "general",
		Topic:     "General discussion",
		Type:      0,
		Position:  0,
	}))
	s.store.Channels.Insert(channelRecord(channelInput{
		ChannelID: "300000000000000002",
		GuildID:   guildID,
		Name:      "random",
		Topic:     "Random stuff",
		Type:      0,
		Position:  1,
	}))
	s.store.Roles.Insert(roleRecord(roleInput{
		RoleID:      guildID,
		GuildID:     guildID,
		Name:        "@everyone",
		Position:    0,
		Permissions: "8",
	}))
	botRole := s.store.Roles.Insert(roleRecord(roleInput{
		RoleID:      "400000000000000001",
		GuildID:     guildID,
		Name:        "Bot",
		Position:    1,
		Permissions: "8",
		Mentionable: true,
	}))
	s.addMemberRole(guildID, botID, stringField(botRole, "role_id"))
	s.store.Tokens.Insert(corestore.Record{
		"token":   "test-token",
		"user_id": botID,
		"scopes":  []string{"bot", "messages.write", "channels.read"},
	})
}

func (s *Service) SeedFromConfig(config SeedConfig) {
	guildID := "100000000000000001"
	if guild := firstRecord(s.store.Guilds.All()); guild != nil {
		guildID = stringField(guild, "guild_id")
	}
	if config.Guild != nil {
		if config.Guild.ID != "" {
			guildID = config.Guild.ID
		}
		name := config.Guild.Name
		if name == "" {
			name = "Emulate"
		}
		if guild := firstRecord(s.store.Guilds.FindBy("guild_id", guildID)); guild != nil {
			s.store.Guilds.Update(intField(guild, "id"), corestore.Record{"name": name})
		} else {
			s.store.Guilds.Insert(corestore.Record{"guild_id": guildID, "name": name, "icon": nil, "owner_id": ""})
		}
	}

	botID := "200000000000000001"
	if bot := s.findFirstBot(); bot != nil {
		botID = stringField(bot, "user_id")
	}
	if config.Bot != nil {
		if config.Bot.ID != "" {
			botID = config.Bot.ID
		}
		username := strings.TrimSpace(config.Bot.Username)
		if username == "" {
			username = "emulate-bot"
		}
		if existing := s.findUser(botID); existing != nil {
			s.store.Users.Update(intField(existing, "id"), corestore.Record{"username": username, "global_name": username, "bot": true})
		} else {
			s.store.Users.Insert(userRecord(userInput{UserID: botID, Username: username, GlobalName: username, Bot: true}))
		}
		token := strings.TrimSpace(config.Bot.Token)
		if token == "" {
			token = generateDiscordToken()
		}
		if existing := firstRecord(s.store.Tokens.FindBy("token", token)); existing == nil {
			s.store.Tokens.Insert(corestore.Record{"token": token, "user_id": botID, "scopes": []string{"bot", "messages.write", "channels.read"}})
		}
	}

	if config.Application != nil {
		applicationID := config.Application.ID
		if applicationID == "" {
			applicationID = "900000000000000001"
		}
		name := config.Application.Name
		if name == "" {
			name = "Emulate Discord App"
		}
		clientID := config.Application.ClientID
		if clientID == "" {
			clientID = applicationID
		}
		clientSecret := config.Application.ClientSecret
		if clientSecret == "" {
			clientSecret = generateDiscordToken()
		}
		redirectURIs := config.Application.RedirectURIs
		if redirectURIs == nil {
			redirectURIs = []string{}
		}
		if app := firstRecord(s.store.Applications.FindBy("application_id", applicationID)); app != nil {
			s.store.Applications.Update(intField(app, "id"), corestore.Record{
				"name":          name,
				"bot_id":        botID,
				"client_id":     clientID,
				"client_secret": clientSecret,
				"redirect_uris": redirectURIs,
				"public_key":    config.Application.PublicKey,
			})
		} else {
			s.store.Applications.Insert(corestore.Record{
				"application_id": applicationID,
				"client_id":      clientID,
				"client_secret":  clientSecret,
				"name":           name,
				"bot_id":         botID,
				"redirect_uris":  redirectURIs,
				"public_key":     config.Application.PublicKey,
			})
		}
	}

	for _, user := range config.Users {
		username := strings.TrimSpace(user.Username)
		if username == "" {
			continue
		}
		userID := user.ID
		if userID == "" {
			userID = generateDiscordID()
		}
		if s.findUser(userID) != nil || firstRecord(s.store.Users.FindBy("username", username)) != nil {
			continue
		}
		globalName := user.GlobalName
		if globalName == "" {
			globalName = username
		}
		s.store.Users.Insert(userRecord(userInput{
			UserID:     userID,
			Username:   username,
			GlobalName: globalName,
			Email:      user.Email,
			Bot:        user.Bot,
		}))
	}

	position := s.store.Channels.Count()
	for _, channel := range config.Channels {
		name := strings.TrimSpace(channel.Name)
		if name == "" || firstRecord(s.store.Channels.FindBy("name", name)) != nil {
			continue
		}
		channelID := channel.ID
		if channelID == "" {
			channelID = generateDiscordID()
		}
		channelGuildID := channel.GuildID
		if channelGuildID == "" {
			channelGuildID = guildID
		}
		s.store.Channels.Insert(channelRecord(channelInput{
			ChannelID: channelID,
			GuildID:   channelGuildID,
			Name:      name,
			Topic:     channel.Topic,
			Type:      channel.Type,
			Position:  position,
		}))
		position++
	}
}

type userInput struct {
	UserID     string
	Username   string
	GlobalName string
	Email      string
	Bot        bool
}

func userRecord(input userInput) corestore.Record {
	globalName := input.GlobalName
	if globalName == "" {
		globalName = input.Username
	}
	return corestore.Record{
		"user_id":       input.UserID,
		"username":      input.Username,
		"discriminator": "0",
		"global_name":   globalName,
		"email":         input.Email,
		"bot":           input.Bot,
		"avatar":        nil,
	}
}

type channelInput struct {
	ChannelID string
	GuildID   string
	Name      string
	Topic     string
	Type      int
	Position  int
}

func channelRecord(input channelInput) corestore.Record {
	return corestore.Record{
		"channel_id":            input.ChannelID,
		"guild_id":              input.GuildID,
		"name":                  input.Name,
		"topic":                 input.Topic,
		"type":                  input.Type,
		"position":              input.Position,
		"nsfw":                  false,
		"last_message_id":       "",
		"parent_id":             nil,
		"permission_overwrites": []any{},
	}
}

func (s *Service) findUser(value string) corestore.Record {
	if value == "" {
		return nil
	}
	if user := firstRecord(s.store.Users.FindBy("user_id", value)); user != nil {
		return user
	}
	return firstRecord(s.store.Users.FindBy("username", value))
}

func (s *Service) findFirstBot() corestore.Record {
	for _, user := range s.store.Users.All() {
		if boolField(user, "bot") {
			return user
		}
	}
	return nil
}

func (s *Service) findGuild(value string) corestore.Record {
	if value == "" {
		return nil
	}
	if guild := firstRecord(s.store.Guilds.FindBy("guild_id", value)); guild != nil {
		return guild
	}
	return firstRecord(s.store.Guilds.FindBy("name", value))
}

func (s *Service) findChannel(value string) corestore.Record {
	if value == "" {
		return nil
	}
	if channel := firstRecord(s.store.Channels.FindBy("channel_id", value)); channel != nil {
		return channel
	}
	return firstRecord(s.store.Channels.FindBy("name", strings.TrimPrefix(value, "#")))
}

func (s *Service) findMessage(channelID string, messageID string) corestore.Record {
	for _, message := range s.store.Messages.FindBy("message_id", messageID) {
		if stringField(message, "channel_id") == channelID {
			return message
		}
	}
	return nil
}

func (s *Service) handleCurrentUser(c *corehttp.Context) {
	user, ok := s.authenticatedUser(c)
	if !ok {
		return
	}
	c.JSON(http.StatusOK, formatUser(user))
}

func (s *Service) handleCurrentApplication(c *corehttp.Context) {
	user, ok := s.authenticatedUser(c)
	if !ok {
		return
	}
	app := firstRecord(s.store.Applications.All())
	if app == nil {
		discordError(c, http.StatusNotFound, "Unknown Application", 10002)
		return
	}
	c.JSON(http.StatusOK, map[string]any{
		"id":                     stringField(app, "application_id"),
		"name":                   stringField(app, "name"),
		"description":            "",
		"verify_key":             stringField(app, "public_key"),
		"bot_public":             true,
		"bot_require_code_grant": false,
		"bot":                    formatUser(user),
	})
}

func (s *Service) handleCurrentUserGuilds(c *corehttp.Context) {
	if _, ok := s.authenticatedUser(c); !ok {
		return
	}
	guilds := make([]map[string]any, 0, s.store.Guilds.Count())
	for _, guild := range s.store.Guilds.All() {
		guilds = append(guilds, map[string]any{
			"id":          stringField(guild, "guild_id"),
			"name":        stringField(guild, "name"),
			"icon":        guild["icon"],
			"owner":       false,
			"permissions": "8",
			"features":    []string{},
		})
	}
	c.JSON(http.StatusOK, guilds)
}

func (s *Service) handleGetGuild(c *corehttp.Context) {
	if _, ok := s.authenticatedUser(c); !ok {
		return
	}
	guild := s.findGuild(c.Param("guildId"))
	if guild == nil {
		discordError(c, http.StatusNotFound, "Unknown Guild", 10004)
		return
	}
	c.JSON(http.StatusOK, formatGuild(guild))
}

func (s *Service) handleGuildChannels(c *corehttp.Context) {
	if _, ok := s.authenticatedUser(c); !ok {
		return
	}
	guild := s.findGuild(c.Param("guildId"))
	if guild == nil {
		discordError(c, http.StatusNotFound, "Unknown Guild", 10004)
		return
	}
	channels := []map[string]any{}
	for _, channel := range s.store.Channels.FindBy("guild_id", stringField(guild, "guild_id")) {
		channels = append(channels, formatChannel(channel))
	}
	sort.SliceStable(channels, func(i int, j int) bool {
		return intValue(channels[i]["position"]) < intValue(channels[j]["position"])
	})
	c.JSON(http.StatusOK, channels)
}

func (s *Service) handleGuildMembers(c *corehttp.Context) {
	if _, ok := s.authenticatedUser(c); !ok {
		return
	}
	guild := s.findGuild(c.Param("guildId"))
	if guild == nil {
		discordError(c, http.StatusNotFound, "Unknown Guild", 10004)
		return
	}
	limit := normalizeLimit(c.Query("limit"), 1, 1000)
	members := []map[string]any{}
	for _, user := range s.store.Users.All() {
		members = append(members, s.formatMemberForGuild(stringField(guild, "guild_id"), user))
	}
	if len(members) > limit {
		members = members[:limit]
	}
	c.JSON(http.StatusOK, members)
}

func (s *Service) handleGuildMember(c *corehttp.Context) {
	if _, ok := s.authenticatedUser(c); !ok {
		return
	}
	guild := s.findGuild(c.Param("guildId"))
	if guild == nil {
		discordError(c, http.StatusNotFound, "Unknown Guild", 10004)
		return
	}
	user := s.findUser(c.Param("userId"))
	if user == nil {
		discordError(c, http.StatusNotFound, "Unknown Member", 10007)
		return
	}
	c.JSON(http.StatusOK, s.formatMemberForGuild(stringField(guild, "guild_id"), user))
}

func (s *Service) handleGetChannel(c *corehttp.Context) {
	if _, ok := s.authenticatedUser(c); !ok {
		return
	}
	channel := s.findChannel(c.Param("channelId"))
	if channel == nil {
		discordError(c, http.StatusNotFound, "Unknown Channel", 10003)
		return
	}
	c.JSON(http.StatusOK, formatChannel(channel))
}

func (s *Service) handleListMessages(c *corehttp.Context) {
	if _, ok := s.authenticatedUser(c); !ok {
		return
	}
	channel := s.findChannel(c.Param("channelId"))
	if channel == nil {
		discordError(c, http.StatusNotFound, "Unknown Channel", 10003)
		return
	}
	limit := normalizeLimit(c.Query("limit"), 50, 100)
	messages := []map[string]any{}
	for _, message := range s.store.Messages.FindBy("channel_id", stringField(channel, "channel_id")) {
		messages = append(messages, s.formatMessage(message))
	}
	sort.SliceStable(messages, func(i int, j int) bool {
		return stringValue(messages[i]["id"]) > stringValue(messages[j]["id"])
	})
	if len(messages) > limit {
		messages = messages[:limit]
	}
	c.JSON(http.StatusOK, messages)
}

func (s *Service) handleCreateMessage(c *corehttp.Context) {
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
	content := stringValue(body["content"])
	embeds := recordSliceValue(body["embeds"])
	attachments := recordSliceValue(body["attachments"])
	if strings.TrimSpace(content) == "" && len(embeds) == 0 && len(attachments) == 0 {
		discordError(c, http.StatusBadRequest, "Cannot send an empty message", 50006)
		return
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	message := s.store.Messages.Insert(corestore.Record{
		"message_id":       generateDiscordID(),
		"channel_id":       stringField(channel, "channel_id"),
		"guild_id":         stringField(channel, "guild_id"),
		"author_id":        stringField(user, "user_id"),
		"content":          content,
		"timestamp":        now,
		"edited_timestamp": nil,
		"tts":              false,
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
	c.JSON(http.StatusOK, s.formatMessage(message))
}

func (s *Service) handleUpdateMessage(c *corehttp.Context) {
	if _, ok := s.authenticatedUser(c); !ok {
		return
	}
	channel := s.findChannel(c.Param("channelId"))
	if channel == nil {
		discordError(c, http.StatusNotFound, "Unknown Channel", 10003)
		return
	}
	message := s.findMessage(stringField(channel, "channel_id"), c.Param("messageId"))
	if message == nil {
		discordError(c, http.StatusNotFound, "Unknown Message", 10008)
		return
	}
	body := parseDiscordBody(c.Request)
	patch := corestore.Record{"edited_timestamp": time.Now().UTC().Format(time.RFC3339Nano)}
	if _, ok := body["content"]; ok {
		patch["content"] = stringValue(body["content"])
	}
	if embeds, ok := body["embeds"]; ok {
		patch["embeds"] = recordSliceValue(embeds)
	}
	updated, _ := s.store.Messages.Update(intField(message, "id"), patch)
	c.JSON(http.StatusOK, s.formatMessage(updated))
}

func (s *Service) handleDeleteMessage(c *corehttp.Context) {
	if _, ok := s.authenticatedUser(c); !ok {
		return
	}
	channel := s.findChannel(c.Param("channelId"))
	if channel == nil {
		discordError(c, http.StatusNotFound, "Unknown Channel", 10003)
		return
	}
	message := s.findMessage(stringField(channel, "channel_id"), c.Param("messageId"))
	if message == nil {
		discordError(c, http.StatusNotFound, "Unknown Message", 10008)
		return
	}
	s.store.Messages.Delete(intField(message, "id"))
	c.Writer.WriteHeader(http.StatusNoContent)
}

func formatUser(user corestore.Record) map[string]any {
	out := map[string]any{
		"id":            stringField(user, "user_id"),
		"username":      stringField(user, "username"),
		"discriminator": stringField(user, "discriminator"),
		"global_name":   stringField(user, "global_name"),
		"avatar":        user["avatar"],
		"bot":           boolField(user, "bot"),
	}
	if email := stringField(user, "email"); email != "" {
		out["email"] = email
	}
	return out
}

func formatGuild(guild corestore.Record) map[string]any {
	return map[string]any{
		"id":                         stringField(guild, "guild_id"),
		"name":                       stringField(guild, "name"),
		"icon":                       guild["icon"],
		"owner_id":                   stringField(guild, "owner_id"),
		"features":                   []string{},
		"preferred_locale":           "en-US",
		"approximate_member_count":   0,
		"approximate_presence_count": 0,
	}
}

func formatChannel(channel corestore.Record) map[string]any {
	return map[string]any{
		"id":                    stringField(channel, "channel_id"),
		"guild_id":              stringField(channel, "guild_id"),
		"name":                  stringField(channel, "name"),
		"topic":                 stringField(channel, "topic"),
		"type":                  intField(channel, "type"),
		"position":              intField(channel, "position"),
		"nsfw":                  boolField(channel, "nsfw"),
		"last_message_id":       stringField(channel, "last_message_id"),
		"parent_id":             channel["parent_id"],
		"permission_overwrites": channel["permission_overwrites"],
	}
}

func (s *Service) formatMember(user corestore.Record) map[string]any {
	guildID := ""
	if guild := firstRecord(s.store.Guilds.All()); guild != nil {
		guildID = stringField(guild, "guild_id")
	}
	return s.formatMemberForGuild(guildID, user)
}

func (s *Service) formatMemberForGuild(guildID string, user corestore.Record) map[string]any {
	return map[string]any{
		"user":                         formatUser(user),
		"nick":                         nil,
		"avatar":                       nil,
		"roles":                        s.memberRoleIDs(guildID, stringField(user, "user_id")),
		"joined_at":                    time.Now().UTC().Format(time.RFC3339Nano),
		"premium_since":                nil,
		"deaf":                         false,
		"mute":                         false,
		"pending":                      false,
		"communication_disabled_until": nil,
	}
}

func (s *Service) formatMessage(message corestore.Record) map[string]any {
	author := s.findUser(stringField(message, "author_id"))
	out := map[string]any{
		"id":               stringField(message, "message_id"),
		"channel_id":       stringField(message, "channel_id"),
		"guild_id":         stringField(message, "guild_id"),
		"author":           formatUser(author),
		"content":          stringField(message, "content"),
		"timestamp":        stringField(message, "timestamp"),
		"edited_timestamp": message["edited_timestamp"],
		"tts":              boolField(message, "tts"),
		"mention_everyone": boolField(message, "mention_everyone"),
		"mentions":         message["mentions"],
		"mention_roles":    message["mention_roles"],
		"attachments":      message["attachments"],
		"embeds":           message["embeds"],
		"pinned":           boolField(message, "pinned"),
		"type":             intField(message, "type"),
	}
	if webhookID := stringField(message, "webhook_id"); webhookID != "" {
		out["webhook_id"] = webhookID
		if webhook := s.findWebhook(webhookID, ""); webhook != nil {
			out["author"] = map[string]any{
				"id":            webhookID,
				"username":      stringField(webhook, "name"),
				"discriminator": "0000",
				"global_name":   stringField(webhook, "name"),
				"avatar":        webhook["avatar"],
				"bot":           true,
			}
		}
	}
	if reactions := formatReactions(message); len(reactions) > 0 {
		out["reactions"] = reactions
	}
	return out
}

func formatReactions(message corestore.Record) []map[string]any {
	reactions := []map[string]any{}
	for _, reaction := range recordSliceValue(message["reactions"]) {
		emoji := stringValue(reaction["name"])
		reactions = append(reactions, map[string]any{
			"count": intValue(reaction["count"]),
			"me":    false,
			"emoji": map[string]any{"id": nil, "name": emoji},
		})
	}
	return reactions
}

func normalizeLimit(value string, fallback int, max int) int {
	limit, err := strconv.Atoi(value)
	if err != nil || limit <= 0 {
		limit = fallback
	}
	if limit > max {
		return max
	}
	return limit
}

func (s *Service) handleInspector(c *corehttp.Context) {
	channels := s.store.Channels.All()
	if len(channels) == 0 {
		c.HTML(http.StatusOK, ui.RenderSettingsPage("Discord Inspector", "<p class=\"empty\">No channels</p>", "<p class=\"empty\">No channels in the emulator store.</p>", ui.PageOptions{Service: serviceLabel}))
		return
	}
	sort.SliceStable(channels, func(i int, j int) bool {
		if stringField(channels[i], "guild_id") == stringField(channels[j], "guild_id") {
			return intField(channels[i], "position") < intField(channels[j], "position")
		}
		return stringField(channels[i], "guild_id") < stringField(channels[j], "guild_id")
	})
	active := channels[0]
	if requested := c.Query("channel"); requested != "" {
		for _, channel := range channels {
			if stringField(channel, "channel_id") == requested {
				active = channel
				break
			}
		}
	}
	inspectorPath := c.Request.URL.Path
	if inspectorPath == "" {
		inspectorPath = "/"
	}
	var sidebar strings.Builder
	for _, channel := range channels {
		className := ""
		if stringField(channel, "channel_id") == stringField(active, "channel_id") {
			className = " class=\"active\""
		}
		sidebar.WriteString("<a href=\"")
		sidebar.WriteString(ui.EscapeAttr(inspectorPath))
		sidebar.WriteString("?channel=")
		sidebar.WriteString(ui.EscapeAttr(stringField(channel, "channel_id")))
		sidebar.WriteString("\"")
		sidebar.WriteString(className)
		sidebar.WriteString("># ")
		sidebar.WriteString(ui.EscapeHTML(stringField(channel, "name")))
		sidebar.WriteString("</a>")
	}
	messages := s.store.Messages.FindBy("channel_id", stringField(active, "channel_id"))
	sort.SliceStable(messages, func(i int, j int) bool {
		return stringField(messages[i], "message_id") > stringField(messages[j], "message_id")
	})
	var messageHTML strings.Builder
	if len(messages) == 0 {
		messageHTML.WriteString("<p class=\"empty\">No messages yet. Post one with the Discord REST API.</p>")
	} else {
		limit := len(messages)
		if limit > 50 {
			limit = 50
		}
		for _, message := range messages[:limit] {
			messageHTML.WriteString(s.renderInspectorMessage(message))
		}
	}
	stats := strconv.Itoa(s.store.Guilds.Count()) + " guilds, " + strconv.Itoa(s.store.Channels.Count()) + " channels, " + strconv.Itoa(s.store.Messages.Count()) + " messages"
	body := "<div class=\"s-card\">" +
		"<div class=\"s-card-header\">" +
		"<div class=\"s-icon\">#</div>" +
		"<div><div class=\"s-title\">" + ui.EscapeHTML(stringField(active, "name")) + "</div>" +
		"<div class=\"s-subtitle\">" + ui.EscapeHTML(firstNonEmpty(stringField(active, "topic"), "No topic set")) + "</div></div>" +
		"</div>" +
		"<div class=\"section-heading\">Messages <span class=\"user-meta\">" + ui.EscapeHTML(stats) + "</span></div>" +
		messageHTML.String() +
		"</div>"
	guildName := "Discord"
	if guild := s.findGuild(stringField(active, "guild_id")); guild != nil {
		guildName = stringField(guild, "name")
	}
	c.HTML(http.StatusOK, ui.RenderSettingsPage(guildName+" Message Inspector", sidebar.String(), body, ui.PageOptions{Service: serviceLabel}))
}

func (s *Service) renderInspectorMessage(message corestore.Record) string {
	author := s.findUser(stringField(message, "author_id"))
	displayName := stringField(author, "username")
	if displayName == "" {
		displayName = stringField(message, "author_id")
	}
	letter := "?"
	if displayName != "" {
		letter = strings.ToUpper(displayName[:1])
	}
	botBadge := ""
	if boolField(author, "bot") {
		botBadge = " <span class=\"badge badge-granted\">bot</span>"
	}
	editedBadge := ""
	if message["edited_timestamp"] != nil {
		editedBadge = " <span class=\"badge badge-requested\">edited</span>"
	}
	return "<div class=\"org-row\">" +
		"<span class=\"org-icon\">" + ui.EscapeHTML(letter) + "</span>" +
		"<span class=\"org-name\">" + ui.EscapeHTML(displayName) + botBadge + "</span>" +
		"<span class=\"user-meta\">" + ui.EscapeHTML(stringField(message, "message_id")) + "</span>" +
		"</div>" +
		"<div class=\"info-text\">" + ui.EscapeHTML(stringField(message, "content")) + editedBadge + "</div>"
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}
