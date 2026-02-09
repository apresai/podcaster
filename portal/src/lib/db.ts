import { DynamoDBClient } from "@aws-sdk/client-dynamodb";
import {
  DynamoDBDocumentClient,
  GetCommand,
  PutCommand,
  QueryCommand,
  UpdateCommand,
} from "@aws-sdk/lib-dynamodb";
import { randomBytes, createHash } from "crypto";

const client = new DynamoDBClient({ region: process.env.AWS_REGION || "us-east-1" });
const ddb = DynamoDBDocumentClient.from(client);
const TABLE = process.env.DYNAMODB_TABLE || "apresai-podcasts-prod";

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
}

export interface Podcast {
  podcastId: string;
  title: string;
  status: string;
  audioUrl?: string;
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
  await ddb.send(
    new PutCommand({
      TableName: TABLE,
      Item: {
        PK: `APIKEY#${prefix}`,
        SK: "METADATA",
        GSI1PK: `USER#${userId}#KEYS`,
        GSI1SK: now,
        userId,
        keyHash,
        name,
        status: "active",
        createdAt: now,
      },
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
  }));
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
      IndexName: "GSI1",
      KeyConditionExpression: "GSI1PK = :pk",
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
