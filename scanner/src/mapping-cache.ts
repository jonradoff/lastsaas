import crypto from "crypto";
import fs from "fs";
import path from "path";
import type { CapabilityName, MappingCacheEntry } from "./types.js";

const CACHE_DIR = ".agentlens";
const CACHE_FILE = "mappings.json";

/**
 * Generate a stable identifier for a server URL or command.
 * Returns the first 16 hex characters of a sha256 hash.
 */
export function getServerIdentifier(urlOrCommand: string): string {
  return crypto.createHash("sha256").update(urlOrCommand).digest("hex").slice(0, 16);
}

function cachePath(projectDir: string): string {
  return path.join(projectDir, CACHE_DIR, CACHE_FILE);
}

/**
 * Read the mapping cache from disk.
 * Returns an empty object if the file is missing or corrupt.
 */
export function readCache(projectDir: string): Record<string, MappingCacheEntry> {
  const filePath = cachePath(projectDir);
  try {
    const raw = fs.readFileSync(filePath, "utf-8");
    const parsed = JSON.parse(raw);
    if (typeof parsed === "object" && parsed !== null && !Array.isArray(parsed)) {
      return parsed as Record<string, MappingCacheEntry>;
    }
    return {};
  } catch {
    return {};
  }
}

/**
 * Write the mapping cache to disk, creating the .agentlens directory if needed.
 */
export function writeCache(
  projectDir: string,
  cache: Record<string, MappingCacheEntry>,
): void {
  const dir = path.join(projectDir, CACHE_DIR);
  if (!fs.existsSync(dir)) {
    fs.mkdirSync(dir, { recursive: true });
  }
  fs.writeFileSync(cachePath(projectDir), JSON.stringify(cache, null, 2), "utf-8");
}

/**
 * Retrieve cached mappings for a specific server, or null if not cached.
 */
export function getCachedMappings(
  projectDir: string,
  serverIdentifier: string,
): Record<CapabilityName, string> | null {
  const cache = readCache(projectDir);
  const entry = cache[serverIdentifier];
  if (!entry) return null;
  return entry.mappings;
}

/**
 * Save capability mappings for a server to the cache.
 */
export function saveMappings(
  projectDir: string,
  serverIdentifier: string,
  mappings: Record<CapabilityName, string>,
): void {
  const cache = readCache(projectDir);
  cache[serverIdentifier] = {
    serverIdentifier,
    mappings,
    timestamp: Date.now(),
  };
  writeCache(projectDir, cache);
}
