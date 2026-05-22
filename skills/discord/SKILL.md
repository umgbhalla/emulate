---
name: discord
description: Emulated Discord REST API for local development and testing. Use when the user needs to interact with Discord API endpoints locally, test bot clients, emulate guilds, channels, members, or messages, or work with Discord REST without hitting the real Discord API. Triggers include "Discord API", "emulate Discord", "mock Discord", "Discord bot", "discord.js REST", "local Discord", or any task requiring a local Discord API.
---

# Discord API Emulator

Stateful Discord REST API emulation with bot auth, guilds, channels, members, messages, seed config, and a message inspector.

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
- `GET /api/v10/users/@me/guilds` - guild list
- `GET /api/v10/guilds/:guildId` - guild detail
- `GET /api/v10/guilds/:guildId/channels` - guild channels
- `GET /api/v10/guilds/:guildId/members` - guild members
- `GET /api/v10/guilds/:guildId/members/:userId` - guild member
- `GET /api/v10/channels/:channelId` - channel detail
- `GET /api/v10/channels/:channelId/messages` - message history
- `POST /api/v10/channels/:channelId/messages` - create message
- `PATCH /api/v10/channels/:channelId/messages/:messageId` - edit message
- `DELETE /api/v10/channels/:channelId/messages/:messageId` - delete message

`/api/v9/*` and `/api/*` aliases are also mounted for common client configurations.

## Seed Config

```yaml
discord:
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

Point REST clients at the emulator base URL and use `Bot test-token` or your seeded bot token. Gateway and slash-command interaction callbacks are not implemented in the native Discord engine yet.
