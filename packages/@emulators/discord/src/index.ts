export const serviceName = "discord";
export const serviceLabel = "Discord REST API";
export const runtime = "native-go";

export interface CompatEntity {
  id: number;
  created_at: string;
  updated_at: string;
  [key: string]: unknown;
}

export type CompatInsertInput<T extends CompatEntity> = Omit<T, "id" | "created_at" | "updated_at"> & { id?: number };

export interface CompatCollection<T extends CompatEntity> {
  all(): T[];
  findBy(field: keyof T, value: unknown): T[];
  insert(input: CompatInsertInput<T>): T;
}

export interface CompatStoreSource {
  collection<T extends CompatEntity>(name: string, indexFields?: (keyof T)[]): CompatCollection<T>;
}

function compatCollection<T extends CompatEntity>(
  store: CompatStoreSource,
  name: string,
  indexFields: (keyof T)[],
): CompatCollection<T> {
  return store.collection<T>(name, indexFields);
}

export interface DiscordApplication extends CompatEntity {
  application_id: string;
  client_id: string;
  client_secret: string;
  name: string;
  bot_id: string;
  redirect_uris: string[];
  public_key: string;
  private_key?: string;
}

export interface DiscordGuild extends CompatEntity {
  guild_id: string;
  name: string;
  icon: string | null;
  owner_id: string;
}

export interface DiscordUser extends CompatEntity {
  user_id: string;
  username: string;
  discriminator: string;
  global_name: string;
  email?: string;
  bot: boolean;
  avatar: string | null;
}

export interface DiscordChannel extends CompatEntity {
  channel_id: string;
  guild_id: string;
  name: string;
  topic: string;
  type: number;
  position: number;
  nsfw: boolean;
  last_message_id: string;
}

export interface DiscordMessage extends CompatEntity {
  message_id: string;
  channel_id: string;
  guild_id: string;
  author_id: string;
  webhook_id?: string;
  content: string;
  timestamp: string;
  edited_timestamp: string | null;
  pinned: boolean;
  type: number;
}

export interface DiscordRole extends CompatEntity {
  role_id: string;
  guild_id: string;
  name: string;
  color: number;
  hoist: boolean;
  position: number;
  permissions: string;
  managed: boolean;
  mentionable: boolean;
}

export interface DiscordMemberRole extends CompatEntity {
  guild_id: string;
  user_id: string;
  role_id: string;
}

export interface DiscordWebhook extends CompatEntity {
  webhook_id: string;
  token: string;
  name: string;
  avatar: string | null;
  channel_id: string;
  guild_id: string;
  application_id: string | null;
  user_id: string;
  type: number;
}

export interface DiscordApplicationCommand extends CompatEntity {
  command_id: string;
  application_id: string;
  guild_id: string;
  name: string;
  description: string;
  type: number;
  options: unknown[];
  default_member_permissions: string | null;
  dm_permission: boolean | null;
  version: string;
}

export interface DiscordToken extends CompatEntity {
  token: string;
  user_id: string;
  scopes: string[];
}

export interface DiscordOAuthCode extends CompatEntity {
  code: string;
  client_id: string;
  redirect_uri: string;
  scope: string;
  state: string;
  user_id: string;
  created_at_ms: number;
}

export interface DiscordSeedConfig {
  port?: number;
  application?: {
    id?: string;
    client_id?: string;
    client_secret?: string;
    name?: string;
    redirect_uris?: string[];
    public_key?: string;
    private_key?: string;
  };
  guild?: { id?: string; name?: string };
  bot?: { id?: string; username?: string; token?: string };
  users?: Array<{ id?: string; username: string; global_name?: string; email?: string; bot?: boolean }>;
  channels?: Array<{ id?: string; guild_id?: string; name: string; topic?: string; type?: number }>;
}

export interface DiscordStore {
  applications: CompatCollection<DiscordApplication>;
  guilds: CompatCollection<DiscordGuild>;
  users: CompatCollection<DiscordUser>;
  channels: CompatCollection<DiscordChannel>;
  messages: CompatCollection<DiscordMessage>;
  roles: CompatCollection<DiscordRole>;
  memberRoles: CompatCollection<DiscordMemberRole>;
  webhooks: CompatCollection<DiscordWebhook>;
  applicationCommands: CompatCollection<DiscordApplicationCommand>;
  oauthCodes: CompatCollection<DiscordOAuthCode>;
  tokens: CompatCollection<DiscordToken>;
}

export function getDiscordStore(store: CompatStoreSource): DiscordStore {
  return {
    applications: compatCollection<DiscordApplication>(store, "discord.applications", ["application_id", "client_id"]),
    guilds: compatCollection<DiscordGuild>(store, "discord.guilds", ["guild_id", "name"]),
    users: compatCollection<DiscordUser>(store, "discord.users", ["user_id", "username", "email"]),
    channels: compatCollection<DiscordChannel>(store, "discord.channels", ["channel_id", "guild_id", "name"]),
    messages: compatCollection<DiscordMessage>(store, "discord.messages", ["message_id", "channel_id"]),
    roles: compatCollection<DiscordRole>(store, "discord.roles", ["role_id", "guild_id", "name"]),
    memberRoles: compatCollection<DiscordMemberRole>(store, "discord.member_roles", ["guild_id", "user_id", "role_id"]),
    webhooks: compatCollection<DiscordWebhook>(store, "discord.webhooks", ["webhook_id", "channel_id", "token"]),
    applicationCommands: compatCollection<DiscordApplicationCommand>(store, "discord.application_commands", [
      "command_id",
      "application_id",
      "guild_id",
    ]),
    oauthCodes: compatCollection<DiscordOAuthCode>(store, "discord.oauth_codes", ["code", "client_id"]),
    tokens: compatCollection<DiscordToken>(store, "discord.tokens", ["token"]),
  };
}

export const service = { name: serviceName, label: serviceLabel, runtime };
export const plugin = service;
export const discordPlugin = plugin;

export function seedFromConfig(_store?: unknown, _baseUrl?: string, _config?: DiscordSeedConfig): void {
  throw new Error("seedFromConfig is not available in @emulators/discord. Pass seed config to createEmulator instead.");
}
