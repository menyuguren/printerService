package web

import (
	"encoding/json"
	"html/template"
	"net/http"
	"sync"

	"printerService/internal/config"
	"printerService/internal/printers"
	"printerService/internal/tasks"
)

type Options struct {
	Config config.Config
	Source printers.Source
	Tasks  *tasks.Store
	Save   func(config.Config) error
}

type Server struct {
	mu     sync.Mutex
	cfg    config.Config
	source printers.Source
	tasks  *tasks.Store
	save   func(config.Config) error
	mux    *http.ServeMux
}

type StatusResponse struct {
	Config        config.Config `json:"config"`
	TargetPrinter printers.Info `json:"target_printer"`
	TargetMode    printers.Mode `json:"target_mode"`
	TargetError   string        `json:"target_error,omitempty"`
	Tasks         []tasks.Task  `json:"tasks"`
}

func NewServer(opts Options) http.Handler {
	if opts.Tasks == nil {
		opts.Tasks = tasks.NewStore(20)
	}
	s := &Server{
		cfg:    opts.Config,
		source: opts.Source,
		tasks:  opts.Tasks,
		save:   opts.Save,
		mux:    http.NewServeMux(),
	}
	s.routes()
	return s
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.mux.ServeHTTP(w, r)
}

func (s *Server) TargetPrinterName() (string, error) {
	s.mu.Lock()
	cfg := s.cfg
	s.mu.Unlock()

	target, _, err := printers.ResolveTarget(cfg, s.source)
	if err != nil {
		return "", err
	}
	return target.Name, nil
}

func (s *Server) routes() {
	s.mux.HandleFunc("/", s.index)
	s.mux.HandleFunc("/api/status", s.status)
	s.mux.HandleFunc("/api/printers", s.listPrinters)
	s.mux.HandleFunc("/api/config", s.configure)
	s.mux.HandleFunc("/api/config/printer", s.configurePrinter)
}

func (s *Server) index(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	noStore(w)
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_ = indexTemplate.Execute(w, nil)
}

func (s *Server) status(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w)
		return
	}
	s.mu.Lock()
	cfg := s.cfg
	s.mu.Unlock()

	target, mode, err := printers.ResolveTarget(cfg, s.source)
	resp := StatusResponse{
		Config:        cfg,
		TargetPrinter: target,
		TargetMode:    mode,
		Tasks:         s.tasks.Recent(),
	}
	if err != nil {
		resp.TargetError = err.Error()
	}
	writeJSON(w, http.StatusOK, resp)
}

func (s *Server) listPrinters(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w)
		return
	}
	list, err := s.source.List()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, list)
}

func (s *Server) configurePrinter(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodPost:
		var req struct {
			PrinterName string `json:"printer_name"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid JSON body")
			return
		}
		if req.PrinterName == "" {
			writeError(w, http.StatusBadRequest, "printer_name is required")
			return
		}
		s.mu.Lock()
		cfg := s.cfg
		cfg.PrinterName = req.PrinterName
		cfg.UseDefaultPrinter = false
		s.cfg = cfg
		s.mu.Unlock()
		if err := s.persist(cfg); err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, cfg)
	case http.MethodDelete:
		s.mu.Lock()
		cfg := s.cfg
		cfg.PrinterName = ""
		cfg.UseDefaultPrinter = true
		s.cfg = cfg
		s.mu.Unlock()
		if err := s.persist(cfg); err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, cfg)
	default:
		methodNotAllowed(w)
	}
}

func (s *Server) configure(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPut {
		methodNotAllowed(w)
		return
	}

	var req config.Config
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	if req.PrinterName == "" {
		req.UseDefaultPrinter = true
	}
	if err := config.Validate(req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	s.mu.Lock()
	s.cfg = req
	s.mu.Unlock()
	if err := s.persist(req); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, req)
}

func (s *Server) persist(cfg config.Config) error {
	if s.save == nil {
		return nil
	}
	return s.save(cfg)
}

func methodNotAllowed(w http.ResponseWriter) {
	writeError(w, http.StatusMethodNotAllowed, "method not allowed")
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	noStore(w)
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}

func noStore(w http.ResponseWriter) {
	w.Header().Set("Cache-Control", "no-store")
}

var indexTemplate = template.Must(template.New("index").Parse(`<!doctype html>
<html lang="zh-CN">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>网络打印机服务</title>
  <style>
    body { margin: 0; font-family: Arial, "Microsoft YaHei", sans-serif; color: #1f2937; background: #f6f7f9; }
    main { max-width: 980px; margin: 0 auto; padding: 24px; }
    h1 { margin: 0 0 20px; font-size: 24px; }
    section { background: #fff; border: 1px solid #e5e7eb; border-radius: 8px; padding: 16px; margin-bottom: 16px; }
    h2 { margin: 0 0 12px; font-size: 16px; }
    dl { display: grid; grid-template-columns: 150px 1fr; gap: 8px 12px; margin: 0; }
    dt { color: #6b7280; }
    dd { margin: 0; }
    table { width: 100%; border-collapse: collapse; }
    th, td { padding: 10px 8px; border-bottom: 1px solid #e5e7eb; text-align: left; }
    label { display: block; color: #374151; font-size: 13px; margin-bottom: 6px; }
    input, select { width: 100%; box-sizing: border-box; border: 1px solid #cbd5e1; border-radius: 6px; padding: 8px 10px; font: inherit; background: #fff; }
    button { border: 1px solid #9ca3af; border-radius: 6px; background: #fff; padding: 8px 12px; cursor: pointer; }
    button.primary { background: #0f766e; color: #fff; border-color: #0f766e; }
    .grid { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 12px; }
    .actions { display: flex; gap: 8px; margin-top: 12px; flex-wrap: wrap; }
    .hint { color: #6b7280; font-size: 13px; margin: 10px 0 0; }
    .error { color: #b91c1c; }
    .success { color: #047857; }
    @media (max-width: 700px) { .grid { grid-template-columns: 1fr; } main { padding: 14px; } table { font-size: 13px; } }
  </style>
</head>
<body>
<main>
  <h1>网络打印机服务</h1>
  <section>
    <h2>状态</h2>
    <dl id="status"></dl>
    <p class="error" id="error"></p>
    <p class="success" id="message"></p>
    <div class="actions">
      <button data-action="refresh">刷新</button>
    </div>
  </section>
  <section>
    <h2>转换设置</h2>
    <div class="grid">
      <div>
        <label for="printer-select">转换打印机</label>
        <select id="printer-select"></select>
      </div>
      <div>
        <label for="printer-mode">选择模式</label>
        <input id="printer-mode" readonly>
      </div>
      <div>
        <label for="control-bind">控制端口绑定地址</label>
        <input id="control-bind" autocomplete="off">
      </div>
      <div>
        <label for="control-port">控制端口</label>
        <input id="control-port" type="number" min="1" max="65535">
      </div>
      <div>
        <label for="data-bind">打印机数据端口绑定地址</label>
        <input id="data-bind" autocomplete="off">
      </div>
      <div>
        <label for="data-port">打印机数据端口</label>
        <input id="data-port" type="number" min="1" max="65535">
      </div>
    </div>
    <p class="hint">端口配置保存后需要重启服务才会切换监听端口；打印机选择保存后立即影响后续任务。</p>
    <div class="actions">
      <button class="primary" data-action="set-printer">保存配置</button>
      <button data-action="clear-printer">恢复系统默认打印机</button>
    </div>
  </section>
  <section>
    <h2>打印机</h2>
    <table>
      <thead><tr><th>名称</th><th>默认</th><th>驱动</th><th>端口</th><th></th></tr></thead>
      <tbody id="printers"></tbody>
    </table>
  </section>
  <section>
    <h2>最近任务</h2>
    <table>
      <thead><tr><th>ID</th><th>来源</th><th>打印机</th><th>大小</th><th>状态</th><th>错误</th></tr></thead>
      <tbody id="tasks"></tbody>
    </table>
  </section>
</main>
<script>
let currentStatus = null;
let printerList = [];

document.addEventListener('click', async event => {
  const button = event.target.closest('button[data-action]');
  if (!button) return;
  const action = button.dataset.action;
  try {
    showMessage('');
    if (action === 'set-printer') await saveConfig();
    if (action === 'clear-printer') await clearPrinter();
    if (action === 'refresh') await loadAll();
  } catch (err) {
    showError(err.message || String(err));
  }
});

async function loadAll() {
  currentStatus = await requestJSON('/api/status');
  printerList = await requestJSON('/api/printers');
  renderStatus(currentStatus);
  renderForm(currentStatus, printerList);
  renderTasks(currentStatus.tasks || []);
  renderPrinters(printerList);
}

function renderStatus(status) {
  showError(status.target_error || '');
  replaceDefinitionList(document.getElementById('status'), [
    ['控制端口', location.host],
    ['数据端口', status.config.data_bind + ':' + status.config.data_port],
    ['目标打印机', status.target_printer.name || '未找到可用打印机'],
    ['选择模式', targetModeText(status.target_mode)]
  ]);
}

function renderForm(status, printers) {
  const config = status.config;
  document.getElementById('control-bind').value = config.control_bind || '127.0.0.1';
  document.getElementById('control-port').value = config.control_port || 8080;
  document.getElementById('data-bind').value = config.data_bind || '0.0.0.0';
  document.getElementById('data-port').value = config.data_port || 9100;
  document.getElementById('printer-mode').value = targetModeText(status.target_mode);

  const selected = config.use_default_printer ? '__default__' : config.printer_name;
  const options = ['<option value="__default__">使用系统默认打印机</option>'].concat((printers || []).map(printer => {
    const label = printer.name + (printer.is_default ? '（系统默认）' : '');
    return '<option value="' + escapeAttr(printer.name) + '">' + escapeHtml(label) + '</option>';
  }));
  const select = document.getElementById('printer-select');
  select.innerHTML = options.join('');
  select.value = selected;
}

function renderPrinters(printers) {
  const tbody = document.getElementById('printers');
  const targetName = currentStatus && currentStatus.target_printer ? currentStatus.target_printer.name : '';
  tbody.innerHTML = (printers || []).map(printer =>
    '<tr><td>' + escapeHtml(printer.name + (printer.name === targetName ? '（当前）' : '')) +
    '</td><td>' + (printer.is_default ? '是' : '') +
    '</td><td>' + escapeHtml(printer.driver || '') +
    '</td><td>' + escapeHtml(printer.port || '') + '</td></tr>'
  ).join('');
}

function renderTasks(tasks) {
  const tbody = document.getElementById('tasks');
  tbody.innerHTML = tasks.map(task =>
    '<tr><td>' + escapeHtml(task.id) +
    '</td><td>' + escapeHtml(task.source_ip) +
    '</td><td>' + escapeHtml(task.printer) +
    '</td><td>' + escapeHtml(task.size) +
    '</td><td>' + escapeHtml(task.status) +
    '</td><td>' + escapeHtml(task.error || '') + '</td></tr>'
  ).join('');
}

async function saveConfig() {
  const selectedPrinter = document.getElementById('printer-select').value;
  const cfg = {
    control_bind: document.getElementById('control-bind').value.trim(),
    control_port: numberValue('control-port'),
    data_bind: document.getElementById('data-bind').value.trim(),
    data_port: numberValue('data-port'),
    printer_name: selectedPrinter === '__default__' ? '' : selectedPrinter,
    use_default_printer: selectedPrinter === '__default__',
    log_level: currentStatus && currentStatus.config.log_level ? currentStatus.config.log_level : 'info'
  };
  await requestJSON('/api/config', {
    method: 'PUT',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(cfg)
  });
  await loadAll();
  showMessage('配置已保存');
}

async function clearPrinter() {
  await requestJSON('/api/config/printer', { method: 'DELETE' });
  await loadAll();
  showMessage('已恢复使用系统默认打印机');
}

async function requestJSON(url, options) {
  const response = await fetch(url, options);
  const body = await response.json();
  if (!response.ok) {
    throw new Error(body.error || response.statusText);
  }
  return body;
}

function replaceDefinitionList(list, pairs) {
  let html = '';
  for (const [key, value] of pairs) {
    html += '<dt>' + escapeHtml(key) + '</dt><dd>' + escapeHtml(value) + '</dd>';
  }
  list.innerHTML = html;
}

function showError(message) {
  document.getElementById('error').textContent = message || '';
}

function showMessage(message) {
  document.getElementById('message').textContent = message || '';
}

function numberValue(id) {
  const value = Number(document.getElementById(id).value);
  if (!Number.isInteger(value) || value < 1 || value > 65535) {
    throw new Error('端口必须是 1 到 65535 之间的整数');
  }
  return value;
}

function targetModeText(mode) {
  if (mode === 'configured') return '用户指定';
  if (mode === 'default') return '系统默认';
  if (mode === 'fallback_default') return '指定失效，回退默认';
  return '-';
}

function escapeHtml(value) {
  return String(value == null ? '' : value).replace(/[&<>"']/g, c => ({
    '&': '&amp;',
    '<': '&lt;',
    '>': '&gt;',
    '"': '&quot;',
    "'": '&#39;'
  }[c]));
}

function escapeAttr(value) {
  return escapeHtml(value).replace(/\x60/g, '&#96;');
}

loadAll().catch(err => showError(err.message || String(err)));
</script>
</body>
</html>`))
