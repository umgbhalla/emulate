# @emulators/discord

Metadata package for the Discord REST API emulator. The service implementation runs in the native Go engine distributed by the `emulate` npm package.

```bash
npm install emulate @emulators/discord
npx emulate --service discord
```

`@emulators/discord` remains importable for package discovery and compatibility, but it does not contain a Node.js service implementation.

Use `createEmulator({ service: "discord" })` from `emulate` or the native CLI to start the service.
