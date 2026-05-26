import * as vscode from 'vscode';
import { isRunning, startDaemon } from './tokenmeter';
import { StatusBarController } from './statusBar';
import { DashboardPanel } from './dashboard';

let statusBar: StatusBarController | undefined;

export async function activate(context: vscode.ExtensionContext): Promise<void> {
  const cfg = vscode.workspace.getConfiguration('tokenmeter');

  if (cfg.get<boolean>('autoStartDaemon', true)) {
    const running = await isRunning();
    if (!running) {
      startDaemon();
      // Give the daemon a moment to bind its port before the first status bar poll.
      await new Promise<void>(resolve => setTimeout(resolve, 1500));
    }
  }

  const pollInterval = cfg.get<number>('pollIntervalSeconds', 10);
  statusBar = new StatusBarController(context);
  statusBar.start(pollInterval);

  context.subscriptions.push(
    vscode.commands.registerCommand('tokenmeter.openDashboard', () =>
      DashboardPanel.show(context)
    ),
    vscode.commands.registerCommand('tokenmeter.startDaemon', async () => {
      startDaemon();
      vscode.window.showInformationMessage('Tokenmeter daemon started.');
    }),
    vscode.commands.registerCommand('tokenmeter.refresh', () => {
      void statusBar?.refresh();
    })
  );
}

export function deactivate(): void {
  statusBar?.stop();
}
