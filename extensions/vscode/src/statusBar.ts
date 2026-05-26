import * as vscode from 'vscode';
import { queryJSON } from './tokenmeter';

export class StatusBarController {
  private readonly item: vscode.StatusBarItem;
  private timer: ReturnType<typeof setInterval> | undefined;

  constructor(context: vscode.ExtensionContext) {
    this.item = vscode.window.createStatusBarItem(vscode.StatusBarAlignment.Right, 100);
    this.item.command = 'tokenmeter.openDashboard';
    this.item.tooltip = 'Tokenmeter — click to open usage dashboard';
    this.item.text = '$(graph-line) tokenmeter';
    this.item.show();
    context.subscriptions.push(this.item);
  }

  start(pollIntervalSeconds: number): void {
    void this.refresh();
    this.timer = setInterval(() => { void this.refresh(); }, pollIntervalSeconds * 1000);
  }

  stop(): void {
    if (this.timer !== undefined) {
      clearInterval(this.timer);
      this.timer = undefined;
    }
  }

  async refresh(): Promise<void> {
    const rows = await queryJSON('1h');
    if (rows.length === 0) {
      this.item.text = '$(graph-line) tokenmeter';
      this.item.tooltip = 'Tokenmeter — no events in the last hour. Click to open dashboard.';
      return;
    }
    const totalTokens = rows.reduce((s, r) => s + r.tokens_input + r.tokens_output, 0);
    const totalCost = rows.reduce((s, r) => s + r.cost_usd, 0);
    const tokensLabel = totalTokens >= 1000
      ? `${(totalTokens / 1000).toFixed(1)}k`
      : String(totalTokens);
    this.item.text = `$(graph-line) ${tokensLabel} tokens · $${totalCost.toFixed(4)}`;
    this.item.tooltip = `Tokenmeter — ${rows.length} requests in the last hour. Click to open dashboard.`;
  }
}
