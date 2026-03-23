import { Client } from "@modelcontextprotocol/sdk/client/index.js";
import { StdioClientTransport } from "@modelcontextprotocol/sdk/client/stdio.js";
import { SSEClientTransport } from "@modelcontextprotocol/sdk/client/sse.js";
import { StreamableHTTPClientTransport } from "@modelcontextprotocol/sdk/client/streamableHttp.js";
import { TOOL_CALL_TIMEOUT_MS } from "./constants.js";

export interface McpConnection {
  client: Client;
  listTools(): Promise<Array<{ name: string; description?: string; inputSchema?: Record<string, unknown> }>>;
  callTool(name: string, params: Record<string, unknown>): Promise<{ content: unknown; durationMs: number }>;
  close(): Promise<void>;
}

function buildConnection(client: Client): McpConnection {
  return {
    client,

    async listTools() {
      const result = await client.listTools();
      return (result.tools ?? []).map((t) => ({
        name: t.name,
        description: t.description,
        inputSchema: t.inputSchema as Record<string, unknown> | undefined,
      }));
    },

    async callTool(name: string, params: Record<string, unknown>) {
      const start = Date.now();

      const timeoutPromise = new Promise<never>((_, reject) => {
        setTimeout(() => reject(new Error(`Tool call '${name}' timed out after ${TOOL_CALL_TIMEOUT_MS}ms`)), TOOL_CALL_TIMEOUT_MS);
      });

      const callPromise = client.callTool({ name, arguments: params });

      const result = await Promise.race([callPromise, timeoutPromise]);
      const durationMs = Date.now() - start;

      // Parse JSON from MCP text content responses
      let content: unknown = result.content;
      if (Array.isArray(result.content)) {
        const textParts = result.content.filter(
          (c: { type: string; text?: string }) => c.type === "text" && typeof c.text === "string",
        );
        if (textParts.length === 1) {
          try {
            content = JSON.parse((textParts[0] as { text: string }).text);
          } catch {
            content = (textParts[0] as { text: string }).text;
          }
        }
      }

      return { content, durationMs };
    },

    async close() {
      await client.close();
    },
  };
}

// Exported alias used by scan command
export { buildConnection as createConnection };

/**
 * Connect to an MCP server via stdio (command line).
 * The command string is split into the executable and its arguments.
 */
export async function connectStdio(command: string): Promise<McpConnection> {
  const parts = command.split(/\s+/);
  const cmd = parts[0];
  const args = parts.slice(1);

  const transport = new StdioClientTransport({ command: cmd, args });

  const client = new Client({
    name: "mcplens",
    version: "0.1.0",
  });

  await client.connect(transport);
  return buildConnection(client);
}

/**
 * Connect to an MCP server via SSE (Server-Sent Events).
 * Falls back to StreamableHTTP if the server doesn't support SSE.
 */
export async function connectSSE(url: string, headers?: Record<string, string>): Promise<McpConnection> {
  const transport = new SSEClientTransport(new URL(url), {
    requestInit: headers ? { headers } : undefined,
  });

  const client = new Client({
    name: "mcplens",
    version: "0.1.0",
  });

  await client.connect(transport);
  return buildConnection(client);
}

/**
 * Connect to an MCP server via Streamable HTTP (POST-based JSON-RPC).
 * This is the transport used by Shopify's MCP endpoints at https://{domain}/api/mcp.
 */
export async function connectHTTP(url: string, headers?: Record<string, string>): Promise<McpConnection> {
  const transport = new StreamableHTTPClientTransport(new URL(url), {
    requestInit: headers ? { headers } : undefined,
  });

  const client = new Client({
    name: "mcplens",
    version: "0.1.0",
  }, { capabilities: {} });

  await client.connect(transport);
  return buildConnection(client);
}
