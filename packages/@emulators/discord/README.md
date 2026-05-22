# @emulators/discord

Metadata package for the Discord REST API emulator. The service implementation runs in the native Go engine distributed by the `emulate` npm package. It covers bot auth, OAuth authorization code flows, guilds, channels, members, roles, messages, reactions, webhooks, application command registration, and signed interaction simulation.

```bash
npm install emulate @emulators/discord
npx emulate --service discord
```

`@emulators/discord` remains importable for package discovery and compatibility, but it does not contain a Node.js service implementation.

Use `createEmulator({ service: "discord" })` from `emulate` or the native CLI to start the service. For app interaction tests, read `GET /_emulate/discord/application` for the public key and post to `/_emulate/discord/interactions` with a `target_url`. Gateway WebSocket connections are not implemented yet.
