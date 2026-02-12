import { DynamoDBClient } from "@aws-sdk/client-dynamodb";
import {
  DynamoDBDocumentClient,
  GetCommand,
  PutCommand,
  QueryCommand,
  UpdateCommand,
} from "@aws-sdk/lib-dynamodb";
import { randomBytes, createHash, createCipheriv, createDecipheriv } from "crypto";

const client = new DynamoDBClient({ region: process.env.AWS_REGION || "us-east-1" });
const ddb = DynamoDBDocumentClient.from(client);
const TABLE = process.env.DYNAMODB_TABLE || "podcaster-prod";

// --- Types ---

export interface User {
  userId: string;
  email: string;
  name: string;
  status: "pending" | "active" | "suspended";
  role: "user" | "admin";
  createdAt: string;
  approvedAt?: string;
}

export interface APIKey {
  prefix: string;
  userId: string;
  keyHash: string;
  name: string;
  status: "active" | "revoked";
  createdAt: string;
  lastUsedAt?: string;
  encryptedKey?: string;
}

export interface Podcast {
  podcastId: string;
  title: string;
  status: string;
  audioUrl?: string;
  scriptUrl?: string;
  model?: string;
  ttsProvider?: string;
  userId?: string;
  estimatedCostUSD?: number;
  duration?: string;
  createdAt: string;
  stage?: string;
  progress?: number;
}

export interface MonthlyUsage {
  month: string;
  podcastCount: number;
  totalDurationSec: number;
  totalTTSChars: number;
  totalCostUSD: number;
}

// --- User operations ---

export async function getUser(userId: string): Promise<User | null> {
  const result = await ddb.send(
    new GetCommand({
      TableName: TABLE,
      Key: { PK: `USER#${userId}`, SK: "PROFILE" },
    })
  );
  if (!result.Item) return null;
  return {
    userId,
    email: result.Item.email,
    name: result.Item.name,
    status: result.Item.status,
    role: result.Item.role || "user",
    createdAt: result.Item.createdAt,
    approvedAt: result.Item.approvedAt,
  };
}

export async function getUserByEmail(email: string): Promise<User | null> {
  const result = await ddb.send(
    new QueryCommand({
      TableName: TABLE,
      IndexName: "GSI1",
      KeyConditionExpression: "GSI1PK = :pk",
      FilterExpression: "email = :email",
      ExpressionAttributeValues: {
        ":pk": "USERS",
        ":email": email,
      },
    })
  );
  if (!result.Items || result.Items.length === 0) return null;
  const item = result.Items[0];
  const userId = (item.PK as string).replace("USER#", "");
  return {
    userId,
    email: item.email,
    name: item.name,
    status: item.status,
    role: item.role || "user",
    createdAt: item.createdAt,
    approvedAt: item.approvedAt,
  };
}

export async function createUser(user: Omit<User, "status" | "role" | "createdAt">): Promise<User> {
  const now = new Date().toISOString();
  const newUser: User = {
    ...user,
    status: "pending",
    role: "user",
    createdAt: now,
  };
  await ddb.send(
    new PutCommand({
      TableName: TABLE,
      Item: {
        PK: `USER#${user.userId}`,
        SK: "PROFILE",
        GSI1PK: "USERS",
        GSI1SK: `${now}#${user.userId}`,
        ...newUser,
      },
      ConditionExpression: "attribute_not_exists(PK)",
    })
  );
  return newUser;
}

export async function listUsers(): Promise<User[]> {
  const result = await ddb.send(
    new QueryCommand({
      TableName: TABLE,
      IndexName: "GSI1",
      KeyConditionExpression: "GSI1PK = :pk",
      ExpressionAttributeValues: { ":pk": "USERS" },
      ScanIndexForward: false,
    })
  );
  return (result.Items || []).map((item) => ({
    userId: (item.PK as string).replace("USER#", ""),
    email: item.email,
    name: item.name,
    status: item.status,
    role: item.role || "user",
    createdAt: item.createdAt,
    approvedAt: item.approvedAt,
  }));
}

export async function approveUser(userId: string): Promise<void> {
  await ddb.send(
    new UpdateCommand({
      TableName: TABLE,
      Key: { PK: `USER#${userId}`, SK: "PROFILE" },
      UpdateExpression: "SET #status = :status, approvedAt = :approvedAt",
      ExpressionAttributeNames: { "#status": "status" },
      ExpressionAttributeValues: {
        ":status": "active",
        ":approvedAt": new Date().toISOString(),
      },
    })
  );
}

export async function suspendUser(userId: string): Promise<void> {
  await ddb.send(
    new UpdateCommand({
      TableName: TABLE,
      Key: { PK: `USER#${userId}`, SK: "PROFILE" },
      UpdateExpression: "SET #status = :status",
      ExpressionAttributeNames: { "#status": "status" },
      ExpressionAttributeValues: { ":status": "suspended" },
    })
  );
}

// --- Encryption helpers ---

function getEncryptionKey(): Buffer {
  const hex = process.env.PORTAL_ENCRYPTION_KEY;
  if (!hex) throw new Error("PORTAL_ENCRYPTION_KEY not configured");
  return Buffer.from(hex, "hex");
}

export function encryptAPIKey(rawKey: string): string {
  const key = getEncryptionKey();
  const iv = randomBytes(12);
  const cipher = createCipheriv("aes-256-gcm", key, iv);
  const encrypted = Buffer.concat([cipher.update(rawKey, "utf8"), cipher.final()]);
  const tag = cipher.getAuthTag();
  return `${iv.toString("base64")}:${encrypted.toString("base64")}:${tag.toString("base64")}`;
}

export function decryptAPIKey(blob: string): string {
  const key = getEncryptionKey();
  const [ivB64, cipherB64, tagB64] = blob.split(":");
  const iv = Buffer.from(ivB64, "base64");
  const encrypted = Buffer.from(cipherB64, "base64");
  const tag = Buffer.from(tagB64, "base64");
  const decipher = createDecipheriv("aes-256-gcm", key, iv);
  decipher.setAuthTag(tag);
  return decipher.update(encrypted) + decipher.final("utf8");
}

// --- API Key operations ---

export function generateAPIKey(): { fullKey: string; prefix: string; keyHash: string } {
  const raw = randomBytes(32).toString("hex");
  const fullKey = `pk_${raw}`;
  const prefix = raw.substring(0, 8);
  const keyHash = createHash("sha256").update(fullKey).digest("hex");
  return { fullKey, prefix, keyHash };
}

export async function createAPIKey(userId: string, name: string): Promise<{ fullKey: string; prefix: string }> {
  const { fullKey, prefix, keyHash } = generateAPIKey();
  const now = new Date().toISOString();
  const item: Record<string, unknown> = {
    PK: `APIKEY#${prefix}`,
    SK: "METADATA",
    GSI1PK: `USER#${userId}#KEYS`,
    GSI1SK: now,
    userId,
    keyHash,
    name,
    status: "active",
    createdAt: now,
  };
  // Store encrypted copy for portal-initiated MCP calls
  if (process.env.PORTAL_ENCRYPTION_KEY) {
    item.encryptedKey = encryptAPIKey(fullKey);
  }
  await ddb.send(
    new PutCommand({
      TableName: TABLE,
      Item: item,
    })
  );
  return { fullKey, prefix };
}

export async function listAPIKeys(userId: string): Promise<APIKey[]> {
  const result = await ddb.send(
    new QueryCommand({
      TableName: TABLE,
      IndexName: "GSI1",
      KeyConditionExpression: "GSI1PK = :pk",
      ExpressionAttributeValues: { ":pk": `USER#${userId}#KEYS` },
      ScanIndexForward: false,
    })
  );
  return (result.Items || []).map((item) => ({
    prefix: (item.PK as string).replace("APIKEY#", ""),
    userId: item.userId,
    keyHash: item.keyHash,
    name: item.name,
    status: item.status,
    createdAt: item.createdAt,
    lastUsedAt: item.lastUsedAt,
    encryptedKey: item.encryptedKey as string | undefined,
  }));
}

export async function getActiveAPIKeyForUser(userId: string): Promise<string | null> {
  const keys = await listAPIKeys(userId);
  const activeKey = keys.find((k) => k.status === "active" && k.encryptedKey);
  if (!activeKey?.encryptedKey) return null;
  return decryptAPIKey(activeKey.encryptedKey);
}

export async function revokeAPIKey(prefix: string): Promise<void> {
  await ddb.send(
    new UpdateCommand({
      TableName: TABLE,
      Key: { PK: `APIKEY#${prefix}`, SK: "METADATA" },
      UpdateExpression: "SET #status = :status",
      ExpressionAttributeNames: { "#status": "status" },
      ExpressionAttributeValues: { ":status": "revoked" },
    })
  );
}

// --- Podcast operations ---

export async function listUserPodcasts(userId: string, limit = 10): Promise<Podcast[]> {
  const result = await ddb.send(
    new QueryCommand({
      TableName: TABLE,
      IndexName: "GSI1",
      KeyConditionExpression: "GSI1PK = :pk",
      ExpressionAttributeValues: { ":pk": `USER#${userId}#PODCASTS` },
      ScanIndexForward: false,
      Limit: limit,
    })
  );
  return (result.Items || []).map(itemToPodcast);
}

export async function listAllPodcasts(limit = 50): Promise<Podcast[]> {
  const result = await ddb.send(
    new QueryCommand({
      TableName: TABLE,
      IndexName: "GSI2",
      KeyConditionExpression: "GSI2PK = :pk",
      ExpressionAttributeValues: { ":pk": "PODCASTS" },
      ScanIndexForward: false,
      Limit: limit,
    })
  );
  return (result.Items || []).map(itemToPodcast);
}

function itemToPodcast(item: Record<string, unknown>): Podcast {
  return {
    podcastId: (item.podcastId as string) || (item.PK as string).replace("PODCAST#", ""),
    title: item.title as string || "Untitled",
    status: item.status as string || "unknown",
    audioUrl: item.audioUrl as string | undefined,
    scriptUrl: item.scriptUrl as string | undefined,
    model: item.model as string | undefined,
    ttsProvider: item.ttsProvider as string | undefined,
    userId: item.userId as string | undefined,
    estimatedCostUSD: item.estimatedCostUSD as number | undefined,
    duration: item.duration as string | undefined,
    createdAt: item.createdAt as string || "",
    stage: item.stage as string | undefined,
    progress: item.progress as number | undefined,
  };
}

// --- Usage operations ---

export async function getMonthlyUsage(userId: string, month: string): Promise<MonthlyUsage | null> {
  const result = await ddb.send(
    new GetCommand({
      TableName: TABLE,
      Key: { PK: `USER#${userId}`, SK: `USAGE#${month}` },
    })
  );
  if (!result.Item) return null;
  return {
    month,
    podcastCount: result.Item.podcastCount || 0,
    totalDurationSec: result.Item.totalDurationSec || 0,
    totalTTSChars: result.Item.totalTTSChars || 0,
    totalCostUSD: result.Item.totalCostUSD || 0,
  };
}

export async function listMonthlyUsage(userId: string): Promise<MonthlyUsage[]> {
  const result = await ddb.send(
    new QueryCommand({
      TableName: TABLE,
      KeyConditionExpression: "PK = :pk AND begins_with(SK, :sk)",
      ExpressionAttributeValues: {
        ":pk": `USER#${userId}`,
        ":sk": "USAGE#",
      },
      ScanIndexForward: false,
    })
  );
  return (result.Items || []).map((item) => ({
    month: (item.SK as string).replace("USAGE#", ""),
    podcastCount: item.podcastCount || 0,
    totalDurationSec: item.totalDurationSec || 0,
    totalTTSChars: item.totalTTSChars || 0,
    totalCostUSD: item.totalCostUSD || 0,
  }));
}

export async function listAllUsage(): Promise<(MonthlyUsage & { userId: string })[]> {
  const users = await listUsers();
  const allUsage: (MonthlyUsage & { userId: string })[] = [];
  for (const user of users) {
    const usage = await listMonthlyUsage(user.userId);
    for (const u of usage) {
      allUsage.push({ ...u, userId: user.userId });
    }
  }
  return allUsage;
}
