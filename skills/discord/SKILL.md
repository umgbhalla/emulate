---
name: discord
description: Emulated Discord REST API for local development and testing. Use when the user needs to interact with Discord API endpoints locally, test bot clients, emulate guilds, channels, members, or messages, or work with Discord REST without hitting the real Discord API. Triggers include "Discord API", "emulate Discord", "mock Discord", "Discord bot", "discord.js REST", "local Discord", or any task requiring a local Discord API.
---

# Discord API Emulator

Stateful Discord REST API emulation with bot auth, guilds, channels, members, roles, messages, reactions, webhooks, application commands, seed config, and a message inspector.

The native Go runtime implements this Discord REST surface for local CLI runs and Vercel Go Function previews. In native CLI runs with multiple services enabled, open `/discord` for the message inspector. When only Discord is enabled, and in Vercel Go Function previews, the inspector is available at the service root. Use `npx emulate vercel init --service discord` for Vercel preview deployments at `/emulate/discord/*`.

## Quick Start

```bash
# Discord only
npx emulate --service discord

# Custom port
npx emulate --service discord --port 4003
```

Default bot auth:

```http
Authorization: Bot test-token
```

## Programmatic Use

```typescript
import { createEmulator } from 'emulate'

const discord = await createEmulator({ service: 'discord', port: 4003 })
// discord.url === 'http://localhost:4003'

await fetch(`${discord.url}/api/v10/channels/300000000000000001/messages`, {
  method: 'POST',
  headers: {
    Authorization: 'Bot test-token',
    'Content-Type': 'application/json',
  },
  body: JSON.stringify({ content: 'hello from local tests' }),
})

await discord.close()
```

## Endpoints

- `GET /api/v10/users/@me` - current bot user
- `GET /api/v10/oauth2/applications/@me` - current application
- `GET /api/v10/gateway` and `GET /api/v10/gateway/bot` - gateway metadata
- `GET /oauth2/authorize` - authorization page with seeded user picker
- `POST /oauth2/authorize/callback` - local authorization callback
- `POST /oauth2/token` and `/api/v10/oauth2/token` - authorization code token exchange
- `GET /api/v10/users/@me/guilds` - guild list
- `POST /api/v10/guilds` - create guild
- `GET /api/v10/guilds/:guildId` - guild detail
- `PATCH /api/v10/guilds/:guildId` - update guild
- `DELETE /api/v10/guilds/:guildId` - delete guild
- `GET /api/v10/guilds/:guildId/channels` - guild channels
- `POST /api/v10/guilds/:guildId/channels` - create channel
- `GET /api/v10/guilds/:guildId/members` - guild members
- `GET /api/v10/guilds/:guildId/members/:userId` - guild member
- `PATCH /api/v10/guilds/:guildId/members/:userId` - update member roles
- `GET /api/v10/guilds/:guildId/roles` - list roles
- `POST /api/v10/guilds/:guildId/roles` - create role
- `PUT /api/v10/guilds/:guildId/members/:userId/roles/:roleId` - add member role
- `DELETE /api/v10/guilds/:guildId/members/:userId/roles/:roleId` - remove member role
- `GET /api/v10/channels/:channelId` - channel detail
- `PATCH /api/v10/channels/:channelId` - update channel
- `DELETE /api/v10/channels/:channelId` - delete channel
- `GET /api/v10/channels/:channelId/messages` - message history
- `GET /api/v10/channels/:channelId/messages/:messageId` - get message
- `POST /api/v10/channels/:channelId/messages` - create message
- `PATCH /api/v10/channels/:channelId/messages/:messageId` - edit message
- `DELETE /api/v10/channels/:channelId/messages/:messageId` - delete message
- `POST /api/v10/channels/:channelId/messages/bulk-delete` - bulk delete messages
- `POST /api/v10/channels/:channelId/typing` - typing indicator
- `GET /api/v10/channels/:channelId/pins` - list pins
- `PUT /api/v10/channels/:channelId/pins/:messageId` - pin message
- `DELETE /api/v10/channels/:channelId/pins/:messageId` - unpin message
- `PUT /api/v10/channels/:channelId/messages/:messageId/reactions/:emoji/@me` - add reaction
- `GET /api/v10/channels/:channelId/messages/:messageId/reactions/:emoji` - list reaction users
- `DELETE /api/v10/channels/:channelId/messages/:messageId/reactions/:emoji/@me` - remove own reaction
- `GET /api/v10/channels/:channelId/webhooks` - list channel webhooks
- `POST /api/v10/channels/:channelId/webhooks` - create channel webhook
- `POST /api/v10/webhooks/:webhookId/:token` - execute webhook
- `GET`, `POST`, `PUT`, `PATCH`, and `DELETE /api/v10/applications/:applicationId/commands` - global application commands
- `GET`, `POST`, `PUT`, `PATCH`, and `DELETE /api/v10/applications/:applicationId/guilds/:guildId/commands` - guild application commands
- `GET /_emulate/discord/application` - local wiring metadata, including application id and public key
- `POST /_emulate/discord/interactions` - send a signed application command interaction to `target_url`

`/api/v9/*` and `/api/*` aliases are also mounted for common client configurations.

## Seed Config

```yaml
discord:
  application:
    client_id: discord-client-id
    client_secret: discord-client-secret
    # Optional 32-byte seed or 64-byte Ed25519 private key in hex.
    # If omitted, emulate supplies a local deterministic key pair.
    private_key: 000102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f
    redirect_uris:
      - http://localhost:3000/api/auth/callback/discord
  guild:
    name: Acme
  bot:
    username: acme-bot
    token: acme-token
  users:
    - username: alice
      global_name: Alice
      email: alice@example.com
  channels:
    - name: general
      topic: General discussion
    - name: ops
      topic: Ops alerts
```

## Client Notes

Point REST clients at the emulator base URL and use `Bot test-token` or your seeded bot token. OAuth clients can use the seeded `discord-client-id` and `discord-client-secret`. Apps that handle Discord interactions should set their public key from `GET /_emulate/discord/application`, then trigger local commands with:

```bash
curl -s -X POST http://localhost:4003/_emulate/discord/interactions \
  -H 'Content-Type: application/json' \
  -d '{"target_url":"http://localhost:3001/api/webhooks/discord","command_name":"ask","content":"hello"}'
```

The interaction simulator signs the payload and creates the matching followup webhook for `/api/v10/webhooks/:applicationId/:interactionToken`. Gateway WebSocket connections are not implemented in the native Discord engine yet.
