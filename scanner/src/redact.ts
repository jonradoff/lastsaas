const AUTH_PATTERNS = ["authorization", "x-api-key", "cookie", "token"];

export function redactHeaders(headers: Record<string, string>): Record<string, string> {
  const redacted: Record<string, string> = {};
  for (const [key, value] of Object.entries(headers)) {
    if (AUTH_PATTERNS.some((p) => key.toLowerCase().includes(p))) {
      redacted[key] = "[REDACTED]";
    } else {
      redacted[key] = value;
    }
  }
  return redacted;
}
