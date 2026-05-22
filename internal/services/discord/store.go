package discord

import corestore "github.com/vercel-labs/emulate/internal/core/store"

type Store struct {
	Applications *corestore.Collection
	Guilds       *corestore.Collection
	Users        *corestore.Collection
	Channels     *corestore.Collection
	Messages     *corestore.Collection
	Tokens       *corestore.Collection
}

func NewStore(store *corestore.Store) Store {
	return Store{
		Applications: store.MustCollection("discord.applications", "application_id"),
		Guilds:       store.MustCollection("discord.guilds", "guild_id", "name"),
		Users:        store.MustCollection("discord.users", "user_id", "username", "email"),
		Channels:     store.MustCollection("discord.channels", "channel_id", "guild_id", "name"),
		Messages:     store.MustCollection("discord.messages", "message_id", "channel_id"),
		Tokens:       store.MustCollection("discord.tokens", "token"),
	}
}
