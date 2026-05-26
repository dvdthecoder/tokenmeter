import * as vscode from 'vscode';
import { execFile, spawn } from 'child_process';
import { promisify } from 'util';

const execFileAsync = promisify(execFile);

export interface Row {
  id: string;
  timestamp: string;
  provider: string;
  model: string;
  username: string;
  client_name: string;
  client_version: string;
  service_tier: string;
  tokens_input: number;
  tokens_output: number;
  tokens_cached: number;
  tokens_cached_creation: number;
  latency_ms: number;
  cost_usd: number;
  streaming: boolean;
}

export function findBinary(): string {
  const configured = vscode.workspace
    .getConfiguration('tokenmeter')
    .get<string>('binaryPath', '');
  return configured || 'tokenmeter';
}

export async function isRunning(): Promise<boolean> {
  try {
    const { stdout } = await execFileAsync(findBinary(), ['status'], { timeout: 3000 });
    return stdout.includes('running');
  } catch {
    return false;
  }
}

export function startDaemon(): void {
  // Fire-and-forget — spawn detached so the daemon outlives the extension host.
  const child = spawn(findBinary(), ['daemon'], { detached: true, stdio: 'ignore' });
  child.unref();
}

export async function queryJSON(last: string): Promise<Row[]> {
  try {
    const { stdout } = await execFileAsync(
      findBinary(),
      ['query', '--last', last, '--format', 'json', '--limit', '1000'],
      { timeout: 5000 }
    );
    const parsed = JSON.parse(stdout);
    return Array.isArray(parsed) ? parsed : [];
  } catch {
    return [];
  }
}
