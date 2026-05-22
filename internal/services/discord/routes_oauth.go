package discord

import (
	"net/http"
	"net/url"
	"strings"
	"time"

	corehttp "github.com/vercel-labs/emulate/internal/core/http"
	corestore "github.com/vercel-labs/emulate/internal/core/store"
	"github.com/vercel-labs/emulate/internal/core/ui"
)

const discordPendingCodeTTL = 10 * time.Minute

func (s *Service) registerOAuthRoutes(router *corehttp.Router) {
	router.Get("/oauth2/authorize", s.handleOAuthAuthorize)
	router.Post("/oauth2/authorize/callback", s.handleOAuthAuthorizeCallback)
	router.Post("/oauth2/token", s.handleOAuthToken)
	router.Post("/api/oauth2/token", s.handleOAuthToken)
	router.Post("/api/v10/oauth2/token", s.handleOAuthToken)
	router.Post("/api/v9/oauth2/token", s.handleOAuthToken)
}

func (s *Service) handleOAuthAuthorize(c *corehttp.Context) {
	clientID := c.Query("client_id")
	redirectURI := c.Query("redirect_uri")
	scope := c.Query("scope")
	state := c.Query("state")

	application := s.findApplicationByClientID(clientID)
	if application == nil {
		c.HTML(http.StatusBadRequest, ui.RenderErrorPage("Application not found", "The client_id '"+clientID+"' is not registered.", ui.PageOptions{Service: serviceLabel}))
		return
	}
	if redirectURI != "" && !matchesRedirectURI(redirectURI, stringSliceValue(application["redirect_uris"])) {
		c.HTML(http.StatusBadRequest, ui.RenderErrorPage("Redirect URI mismatch", "The redirect_uri is not registered for this application.", ui.PageOptions{Service: serviceLabel}))
		return
	}

	var body strings.Builder
	count := 0
	for _, user := range s.store.Users.All() {
		if boolField(user, "bot") {
			continue
		}
		username := stringField(user, "username")
		letter := "?"
		if username != "" {
			letter = strings.ToUpper(username[:1])
		}
		body.WriteString(ui.RenderUserButton(ui.UserButtonOptions{
			Letter:     letter,
			Login:      username,
			Name:       stringField(user, "global_name"),
			Email:      stringField(user, "email"),
			FormAction: "/oauth2/authorize/callback",
			HiddenFields: map[string]string{
				"user_id":      stringField(user, "user_id"),
				"client_id":    clientID,
				"redirect_uri": redirectURI,
				"scope":        scope,
				"state":        state,
			},
		}))
		count++
	}
	if count == 0 {
		body.WriteString("<p class=\"empty\">No users in the emulator store.</p>")
	}
	subtitle := "Authorize <strong>" + ui.EscapeHTML(stringField(application, "name")) + "</strong> to access your Discord account."
	c.HTML(http.StatusOK, ui.RenderCardPage("Sign in to Discord", subtitle, body.String(), ui.PageOptions{Service: serviceLabel}))
}

func (s *Service) handleOAuthAuthorizeCallback(c *corehttp.Context) {
	if err := c.Request.ParseForm(); err != nil {
		c.JSON(http.StatusBadRequest, map[string]any{"error": "invalid_request"})
		return
	}
	userID := c.Request.Form.Get("user_id")
	clientID := c.Request.Form.Get("client_id")
	redirectURI := c.Request.Form.Get("redirect_uri")
	scope := c.Request.Form.Get("scope")
	state := c.Request.Form.Get("state")

	application := s.findApplicationByClientID(clientID)
	if application == nil || (redirectURI != "" && !matchesRedirectURI(redirectURI, stringSliceValue(application["redirect_uris"]))) {
		c.JSON(http.StatusBadRequest, map[string]any{"error": "invalid_client"})
		return
	}
	if user := s.findUser(userID); user == nil || boolField(user, "bot") {
		c.JSON(http.StatusBadRequest, map[string]any{"error": "invalid_grant"})
		return
	}
	target, err := url.Parse(redirectURI)
	if err != nil || target == nil || target.Scheme == "" {
		c.JSON(http.StatusBadRequest, map[string]any{"error": "invalid_redirect_uri"})
		return
	}
	code := generateDiscordToken()
	s.store.OAuthCodes.Insert(corestore.Record{
		"code":          code,
		"client_id":     clientID,
		"redirect_uri":  redirectURI,
		"scope":         scope,
		"state":         state,
		"user_id":       userID,
		"created_at_ms": time.Now().UnixMilli(),
	})
	query := target.Query()
	query.Set("code", code)
	if state != "" {
		query.Set("state", state)
	}
	target.RawQuery = query.Encode()
	c.Redirect(http.StatusFound, target.String())
}

func (s *Service) handleOAuthToken(c *corehttp.Context) {
	body := parseDiscordOAuthBody(c.Request)
	clientID := stringValue(body["client_id"])
	clientSecret := stringValue(body["client_secret"])
	if basicClientID, basicClientSecret, ok := basicAuthCredentials(c.Header("Authorization")); ok {
		clientID = basicClientID
		clientSecret = basicClientSecret
	}
	grantType := stringValue(body["grant_type"])
	if grantType == "" {
		grantType = "authorization_code"
	}
	if grantType != "authorization_code" {
		c.JSON(http.StatusBadRequest, map[string]any{"error": "unsupported_grant_type"})
		return
	}
	application := s.findApplicationByClientID(clientID)
	if application == nil || !constantTimeEqual(clientSecret, stringField(application, "client_secret")) {
		c.JSON(http.StatusUnauthorized, map[string]any{"error": "invalid_client"})
		return
	}
	codeValue := stringValue(body["code"])
	code := firstRecord(s.store.OAuthCodes.FindBy("code", codeValue))
	if code == nil || time.Now().UnixMilli()-int64(intField(code, "created_at_ms")) > discordPendingCodeTTL.Milliseconds() {
		if code != nil {
			s.store.OAuthCodes.Delete(intField(code, "id"))
		}
		c.JSON(http.StatusBadRequest, map[string]any{"error": "invalid_grant"})
		return
	}
	if stringField(code, "client_id") != clientID {
		c.JSON(http.StatusBadRequest, map[string]any{"error": "invalid_grant"})
		return
	}
	if redirectURI := stringValue(body["redirect_uri"]); redirectURI != "" && redirectURI != stringField(code, "redirect_uri") {
		c.JSON(http.StatusBadRequest, map[string]any{"error": "invalid_grant"})
		return
	}
	s.store.OAuthCodes.Delete(intField(code, "id"))
	user := s.findUser(stringField(code, "user_id"))
	if user == nil {
		c.JSON(http.StatusBadRequest, map[string]any{"error": "invalid_grant"})
		return
	}
	accessToken := "discord_" + strings.TrimPrefix(generateDiscordToken(), "bot-")
	scope := stringField(code, "scope")
	if scope == "" {
		scope = "identify guilds"
	}
	s.store.Tokens.Insert(corestore.Record{
		"token":   accessToken,
		"user_id": stringField(user, "user_id"),
		"scopes":  strings.Fields(scope),
	})
	c.JSON(http.StatusOK, map[string]any{
		"access_token":  accessToken,
		"token_type":    "Bearer",
		"expires_in":    604800,
		"refresh_token": "discord_refresh_" + strings.TrimPrefix(generateDiscordToken(), "bot-"),
		"scope":         scope,
		"user":          formatUser(user),
	})
}

func (s *Service) findApplicationByClientID(clientID string) corestore.Record {
	if clientID == "" {
		return nil
	}
	if application := firstRecord(s.store.Applications.FindBy("client_id", clientID)); application != nil {
		return application
	}
	return firstRecord(s.store.Applications.FindBy("application_id", clientID))
}
