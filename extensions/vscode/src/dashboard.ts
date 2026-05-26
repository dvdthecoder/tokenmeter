import * as vscode from 'vscode';
import { queryJSON, Row } from './tokenmeter';

export class DashboardPanel {
  private static current: DashboardPanel | undefined;

  private readonly panel: vscode.WebviewPanel;
  private readonly disposables: vscode.Disposable[] = [];

  static async show(context: vscode.ExtensionContext): Promise<void> {
    if (DashboardPanel.current) {
      DashboardPanel.current.panel.reveal(vscode.ViewColumn.One);
      await DashboardPanel.current.update();
      return;
    }
    const panel = vscode.window.createWebviewPanel(
      'tokenmeter.dashboard',
      'Tokenmeter Dashboard',
      vscode.ViewColumn.One,
      { enableScripts: true, retainContextWhenHidden: true }
    );
    DashboardPanel.current = new DashboardPanel(panel);
    await DashboardPanel.current.update();
  }

  private constructor(panel: vscode.WebviewPanel) {
    this.panel = panel;
    this.panel.onDidDispose(() => this.dispose(), null, this.disposables);
    this.panel.webview.onDidReceiveMessage(
      async (msg: { command: string }) => {
        if (msg.command === 'refresh') {
          await this.update();
        }
      },
      null,
      this.disposables
    );
  }

  async update(): Promise<void> {
    const [rows24h, rows7d] = await Promise.all([
      queryJSON('24h'),
      queryJSON('7d'),
    ]);
    this.panel.webview.html = buildHtml(rows24h, rows7d);
  }

  private dispose(): void {
    DashboardPanel.current = undefined;
    this.panel.dispose();
    for (const d of this.disposables) { d.dispose(); }
    this.disposables.length = 0;
  }
}

function buildHtml(rows24h: Row[], rows7d: Row[]): string {
  // Aggregate tokens by model (24h)
  const byModel: Record<string, { input: number; output: number; cached: number }> = {};
  for (const r of rows24h) {
    if (!byModel[r.model]) { byModel[r.model] = { input: 0, output: 0, cached: 0 }; }
    byModel[r.model].input += r.tokens_input;
    byModel[r.model].output += r.tokens_output;
    byModel[r.model].cached += r.tokens_cached;
  }

  // Aggregate cost by calendar day (7d)
  const byDay: Record<string, number> = {};
  for (const r of rows7d) {
    const day = r.timestamp.slice(0, 10);
    byDay[day] = (byDay[day] ?? 0) + r.cost_usd;
  }
  const sortedDays = Object.keys(byDay).sort();

  const totalCost = rows24h.reduce((s, r) => s + r.cost_usd, 0);
  const totalTokens = rows24h.reduce((s, r) => s + r.tokens_input + r.tokens_output, 0);

  const tableRows = rows24h.slice(0, 20).map(r => `
    <tr>
      <td>${new Date(r.timestamp).toLocaleTimeString()}</td>
      <td>${esc(r.provider)}</td>
      <td>${esc(r.model)}</td>
      <td>${(r.tokens_input + r.tokens_output).toLocaleString()}</td>
      <td>$${r.cost_usd.toFixed(6)}</td>
      <td>${r.latency_ms}ms</td>
    </tr>`).join('');

  const emptyRow = '<tr><td colspan="6" style="opacity:0.5;text-align:center;padding:20px">No events in the last 24 hours</td></tr>';

  return `<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="UTF-8">
  <meta http-equiv="Content-Security-Policy" content="default-src 'none'; script-src 'unsafe-inline'; style-src 'unsafe-inline';">
  <style>
    * { box-sizing: border-box; margin: 0; padding: 0; }
    body {
      font-family: var(--vscode-font-family, sans-serif);
      font-size: var(--vscode-font-size, 13px);
      color: var(--vscode-foreground);
      background: var(--vscode-editor-background);
      padding: 20px 24px;
    }
    .header { display: flex; justify-content: space-between; align-items: center; margin-bottom: 4px; }
    h1 { font-size: 1.3em; font-weight: 600; }
    .subtitle { opacity: 0.55; font-size: 0.8em; margin-bottom: 16px; }
    .stats { display: flex; gap: 16px; margin-bottom: 20px; flex-wrap: wrap; }
    .stat {
      background: var(--vscode-sideBar-background, #1e1e1e);
      border-radius: 6px;
      padding: 12px 20px;
      min-width: 120px;
    }
    .stat-value { font-size: 1.5em; font-weight: 700; color: var(--vscode-textLink-foreground, #4fc1ff); }
    .stat-label { font-size: 0.78em; opacity: 0.65; margin-top: 3px; }
    .charts { display: grid; grid-template-columns: 1fr 1fr; gap: 16px; margin-bottom: 20px; }
    @media (max-width: 600px) { .charts { grid-template-columns: 1fr; } }
    .chart-box {
      background: var(--vscode-sideBar-background, #1e1e1e);
      border-radius: 6px;
      padding: 14px 16px;
    }
    .chart-box h3 { font-size: 0.88em; opacity: 0.7; font-weight: 500; margin-bottom: 10px; }
    canvas { display: block; width: 100%; }
    h3.section { font-size: 0.95em; font-weight: 600; margin-bottom: 8px; }
    table { width: 100%; border-collapse: collapse; font-size: 0.82em; }
    th {
      text-align: left;
      padding: 6px 8px;
      border-bottom: 1px solid var(--vscode-panel-border, rgba(128,128,128,0.3));
      opacity: 0.6;
      font-weight: 500;
    }
    td {
      padding: 5px 8px;
      border-bottom: 1px solid var(--vscode-panel-border, rgba(128,128,128,0.15));
    }
    .legend { display: flex; gap: 14px; margin-top: 6px; font-size: 0.75em; opacity: 0.7; }
    .legend-dot { display: inline-block; width: 8px; height: 8px; border-radius: 50%; margin-right: 4px; }
    button {
      background: var(--vscode-button-background, #0e639c);
      color: var(--vscode-button-foreground, #fff);
      border: none;
      padding: 5px 12px;
      border-radius: 4px;
      cursor: pointer;
      font-size: 0.82em;
    }
    button:hover { background: var(--vscode-button-hoverBackground, #1177bb); }
  </style>
</head>
<body>
  <div class="header">
    <h1>Tokenmeter Dashboard</h1>
    <button onclick="refresh()">↻ Refresh</button>
  </div>
  <p class="subtitle">Last 24 hours · ${rows24h.length} requests</p>

  <div class="stats">
    <div class="stat">
      <div class="stat-value">${totalTokens.toLocaleString()}</div>
      <div class="stat-label">Total tokens</div>
    </div>
    <div class="stat">
      <div class="stat-value">$${totalCost.toFixed(4)}</div>
      <div class="stat-label">Estimated cost</div>
    </div>
    <div class="stat">
      <div class="stat-value">${rows24h.length}</div>
      <div class="stat-label">Requests</div>
    </div>
  </div>

  <div class="charts">
    <div class="chart-box">
      <h3>Tokens by Model (24h)</h3>
      <canvas id="modelChart" height="180"></canvas>
      <div class="legend">
        <span><span class="legend-dot" style="background:#4fc1ff"></span>Input</span>
        <span><span class="legend-dot" style="background:#9ccc65"></span>Output</span>
        <span><span class="legend-dot" style="background:#ffb74d"></span>Cached</span>
      </div>
    </div>
    <div class="chart-box">
      <h3>Cost over Time (7d)</h3>
      <canvas id="costChart" height="180"></canvas>
    </div>
  </div>

  <h3 class="section">Recent Requests</h3>
  <table>
    <thead>
      <tr><th>Time</th><th>Provider</th><th>Model</th><th>Tokens</th><th>Cost</th><th>Latency</th></tr>
    </thead>
    <tbody>${tableRows || emptyRow}</tbody>
  </table>

  <script>
    const vscode = acquireVsCodeApi();
    function refresh() { vscode.postMessage({ command: 'refresh' }); }

    const MODEL_LABELS = ${JSON.stringify(Object.keys(byModel))};
    const INPUT_DATA   = ${JSON.stringify(Object.keys(byModel).map(m => byModel[m].input))};
    const OUTPUT_DATA  = ${JSON.stringify(Object.keys(byModel).map(m => byModel[m].output))};
    const CACHED_DATA  = ${JSON.stringify(Object.keys(byModel).map(m => byModel[m].cached))};
    const DAY_LABELS   = ${JSON.stringify(sortedDays)};
    const COST_DATA    = ${JSON.stringify(sortedDays.map(d => byDay[d]))};

    const FG = getComputedStyle(document.body).color || '#ccc';
    const LINK = getComputedStyle(document.body).getPropertyValue('--vscode-textLink-foreground').trim() || '#4fc1ff';

    function drawBarChart(id, labels, datasets) {
      const canvas = document.getElementById(id);
      if (!canvas || labels.length === 0) return;
      const dpr = window.devicePixelRatio || 1;
      const W = canvas.parentElement.clientWidth - 32;
      const H = 180;
      canvas.style.width  = W + 'px';
      canvas.style.height = H + 'px';
      canvas.width  = W * dpr;
      canvas.height = H * dpr;
      const ctx = canvas.getContext('2d');
      ctx.scale(dpr, dpr);

      const pad = { top: 10, right: 10, bottom: 36, left: 52 };
      const cW = W - pad.left - pad.right;
      const cH = H - pad.top - pad.bottom;
      const maxVal = Math.max(...datasets.flatMap(d => d.data), 1);
      const n = labels.length;
      const groupW = cW / n;
      const barW = Math.max(1, (groupW - 6) / datasets.length);

      ctx.font = '9px sans-serif';
      ctx.fillStyle = FG;
      ctx.textAlign = 'right';
      for (let i = 0; i <= 4; i++) {
        const val = maxVal * i / 4;
        const y = pad.top + cH - (cH * i / 4);
        ctx.fillText(val >= 1000 ? (val / 1000).toFixed(1) + 'k' : Math.round(val).toString(), pad.left - 4, y + 3);
        ctx.strokeStyle = 'rgba(128,128,128,0.15)';
        ctx.beginPath(); ctx.moveTo(pad.left, y); ctx.lineTo(pad.left + cW, y); ctx.stroke();
      }

      datasets.forEach((ds, di) => {
        ctx.fillStyle = ds.color;
        labels.forEach((_, i) => {
          const val = ds.data[i] || 0;
          const bH = (val / maxVal) * cH;
          const x = pad.left + i * groupW + di * barW + 3;
          const y = pad.top + cH - bH;
          ctx.fillRect(x, y, barW - 1, bH);
        });
      });

      ctx.fillStyle = FG;
      ctx.textAlign = 'center';
      ctx.font = '9px sans-serif';
      labels.forEach((lbl, i) => {
        const x = pad.left + i * groupW + groupW / 2;
        const short = lbl.length > 14 ? '…' + lbl.slice(-13) : lbl;
        ctx.fillText(short, x, H - pad.bottom + 14);
      });
    }

    function drawLineChart(id, labels, data) {
      const canvas = document.getElementById(id);
      if (!canvas) return;
      const dpr = window.devicePixelRatio || 1;
      const W = canvas.parentElement.clientWidth - 32;
      const H = 180;
      canvas.style.width  = W + 'px';
      canvas.style.height = H + 'px';
      canvas.width  = W * dpr;
      canvas.height = H * dpr;
      const ctx = canvas.getContext('2d');
      ctx.scale(dpr, dpr);

      const pad = { top: 10, right: 10, bottom: 36, left: 60 };
      const cW = W - pad.left - pad.right;
      const cH = H - pad.top - pad.bottom;
      const maxVal = Math.max(...data, 0.000001);
      const n = data.length;

      ctx.font = '9px sans-serif';
      ctx.fillStyle = FG;
      ctx.textAlign = 'right';
      for (let i = 0; i <= 4; i++) {
        const val = maxVal * i / 4;
        const y = pad.top + cH - (cH * i / 4);
        ctx.fillText('$' + val.toFixed(4), pad.left - 4, y + 3);
        ctx.strokeStyle = 'rgba(128,128,128,0.15)';
        ctx.beginPath(); ctx.moveTo(pad.left, y); ctx.lineTo(pad.left + cW, y); ctx.stroke();
      }

      if (n > 0) {
        const px = (i) => pad.left + (n <= 1 ? cW / 2 : (i / (n - 1)) * cW);
        const py = (v) => pad.top + cH - (v / maxVal) * cH;

        // Fill under line
        ctx.fillStyle = LINK + '28';
        ctx.beginPath();
        ctx.moveTo(px(0), py(data[0]));
        for (let i = 1; i < n; i++) ctx.lineTo(px(i), py(data[i]));
        ctx.lineTo(px(n - 1), pad.top + cH);
        ctx.lineTo(px(0), pad.top + cH);
        ctx.closePath();
        ctx.fill();

        // Line
        ctx.strokeStyle = LINK;
        ctx.lineWidth = 2;
        ctx.beginPath();
        ctx.moveTo(px(0), py(data[0]));
        for (let i = 1; i < n; i++) ctx.lineTo(px(i), py(data[i]));
        ctx.stroke();

        // Dots
        ctx.fillStyle = LINK;
        for (let i = 0; i < n; i++) {
          ctx.beginPath(); ctx.arc(px(i), py(data[i]), 3, 0, Math.PI * 2); ctx.fill();
        }
      }

      // X labels
      ctx.fillStyle = FG;
      ctx.textAlign = 'center';
      ctx.font = '9px sans-serif';
      labels.forEach((lbl, i) => {
        const x = n <= 1 ? pad.left + cW / 2 : pad.left + (i / (n - 1)) * cW;
        ctx.fillText(lbl.slice(5), x, H - pad.bottom + 14); // show MM-DD
      });
    }

    window.addEventListener('load', () => {
      drawBarChart('modelChart', MODEL_LABELS, [
        { data: INPUT_DATA,  color: '#4fc1ff' },
        { data: OUTPUT_DATA, color: '#9ccc65' },
        { data: CACHED_DATA, color: '#ffb74d' },
      ]);
      drawLineChart('costChart', DAY_LABELS, COST_DATA);
    });
  </script>
</body>
</html>`;
}

function esc(s: string): string {
  return s.replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;');
}
