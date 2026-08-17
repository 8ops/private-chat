(function () {
  'use strict';
  const ME = window.__USER__ || { id: '', username: 'me', nickname: 'me', role: 'user' };
  const $ = (id) => document.getElementById(id);

  // ---------- 用户信息 ----------
  $('meName').textContent = ME.nickname || ME.username;
  $('meRole').textContent = ME.role === 'admin' ? '管理员' : '普通用户';

  // ---------- DOM ----------
  const messagesEl = $('messages');
  const inputEl = $('input');
  const connStatus = $('connStatus');
  const onlineCount = $('onlineCount');
  const userListEl = $('userList');
  const panel = $('panel');
  const panelTabs = $('panelTabs');
  const panelBody = $('panelBody');

  // ---------- WebSocket ----------
  let ws = null;
  let retry = 0;
  let closing = false;
  const fileMetaCache = {};

  function wsURL() {
    const proto = location.protocol === 'https:' ? 'wss:' : 'ws:';
    return proto + '//' + location.host + '/ws';
  }

  function connect() {
    connStatus.textContent = '连接中…';
    connStatus.className = 'conn-status off';
    ws = new WebSocket(wsURL());
    ws.onopen = function () {
      retry = 0;
      connStatus.textContent = '已连接';
      connStatus.className = 'conn-status';
      loadRecent();
    };
    ws.onmessage = function (ev) {
      let env;
      try { env = JSON.parse(ev.data); } catch (e) { return; }
      if (env.type === 'message') { renderMessage(env.data, false); }
      else if (env.type === 'presence') { renderPresence(env.data); }
      else if (env.type === 'pong') { /* heartbeat */ }
      else if (env.type === 'error') { console.warn('server error', env.data); }
      else if (env.type === 'kicked') { location.href = '/login'; }
    };
    ws.onclose = function () {
      if (closing) return;
      connStatus.textContent = '连接已断开，正在重新连接…';
      connStatus.className = 'conn-status off';
      const delay = Math.min(1000 * Math.pow(2, retry), 30000);
      retry++;
      setTimeout(connect, delay);
    };
    ws.onerror = function () { try { ws.close(); } catch (e) {} };
  }

  function send(msg) {
    if (ws && ws.readyState === WebSocket.OPEN) {
      ws.send(JSON.stringify({ type: 'message', data: msg }));
    }
  }

  // 应用层心跳：每 30 秒 ping。
  setInterval(function () {
    if (ws && ws.readyState === WebSocket.OPEN) {
      ws.send(JSON.stringify({ type: 'ping' }));
    }
  }, 30000);

  // ---------- 加载最近消息 ----------
  async function loadRecent() {
    try {
      const r = await fetch('/api/messages?limit=200');
      const data = await r.json();
      if (data.code === 0 && Array.isArray(data.data)) {
        messagesEl.innerHTML = '';
        data.data.forEach((m) => renderMessage(m, true));
        scrollBottom();
      }
    } catch (e) {}
  }

  // ---------- 渲染消息 ----------
  function renderMessage(m, isHistory) {
    const wrap = document.createElement('div');
    wrap.className = 'msg' + (m.sender_id === ME.id ? ' self' : '');

    const meta = document.createElement('div');
    meta.className = 'meta';
    const t = new Date(m.created_at);
    meta.textContent = (m.sender_name || '匿名') + '  ' + fmtTime(t);
    wrap.appendChild(meta);

    const bubble = document.createElement('div');
    bubble.className = 'bubble';

    if (m.message_type === 'sticker') {
      const img = document.createElement('img');
      img.className = 'msg-img';
      img.src = '/static/stickers/' + m.content + '.svg';
      img.onerror = function () {
        img.style.cssText = 'width:80px;height:80px;background:#eee;border-radius:8px;';
      };
      bubble.appendChild(img);
    } else if (m.message_type === 'image') {
      renderFile(m, bubble, true);
    } else if (m.message_type === 'file') {
      renderFile(m, bubble, false);
    } else {
      // text / emoji：使用 textContent 防止 XSS
      bubble.textContent = m.content;
    }
    wrap.appendChild(bubble);
    messagesEl.appendChild(wrap);
    if (!isHistory) scrollBottom();
  }

  async function renderFile(m, bubble, isImage) {
    const meta = await getFileMeta(m.file_id);
    if (!meta) {
      bubble.textContent = '[文件已过期或不存在]';
      return;
    }
    if (isImage) {
      const img = document.createElement('img');
      img.className = 'msg-img';
      img.src = meta.url;
      img.onclick = function () { window.open(meta.url, '_blank'); };
      bubble.appendChild(img);
    } else {
      const card = document.createElement('div');
      card.className = 'file-msg';
      const icon = document.createElement('div'); icon.className = 'fi'; icon.textContent = '📄';
      const info = document.createElement('div');
      const name = document.createElement('div'); name.className = 'fname'; name.textContent = meta.original_name;
      const fm = document.createElement('div'); fm.className = 'fmeta'; fm.textContent = fmtSize(meta.size);
      info.appendChild(name); info.appendChild(fm);
      const dl = document.createElement('a');
      dl.className = 'dl mini-btn'; dl.href = meta.download_url; dl.textContent = '下载'; dl.setAttribute('download', '');
      card.appendChild(icon); card.appendChild(info); card.appendChild(dl);
      bubble.appendChild(card);
    }
  }

  function getFileMeta(id) {
    if (fileMetaCache[id]) return Promise.resolve(fileMetaCache[id]);
    return fetch('/api/files/' + id).then(r => r.json()).then(d => {
      if (d.code === 0) { fileMetaCache[id] = d.data; return d.data; }
      return null;
    }).catch(() => null);
  }

  function renderPresence(p) {
    if (!p) return;
    onlineCount.textContent = p.online_count || 0;
    userListEl.innerHTML = '';
    (p.users || []).forEach(function (u) {
      const item = document.createElement('div');
      item.className = 'user-item';
      const av = document.createElement('div');
      av.className = 'avatar';
      av.textContent = (u.nickname || u.username || '?').slice(0, 1).toUpperCase();
      const nm = document.createElement('div');
      nm.textContent = u.nickname || u.username;
      item.appendChild(av); item.appendChild(nm);
      userListEl.appendChild(item);
    });
  }

  function scrollBottom() { messagesEl.scrollTop = messagesEl.scrollHeight; }
  function fmtTime(d) {
    const p = (n) => (n < 10 ? '0' + n : '' + n);
    return p(d.getHours()) + ':' + p(d.getMinutes());
  }
  function fmtSize(n) {
    if (n < 1024) return n + ' B';
    if (n < 1024 * 1024) return (n / 1024).toFixed(1) + ' KB';
    return (n / 1024 / 1024).toFixed(1) + ' MB';
  }

  // ---------- 发送文本/表情 ----------
  function sendText() {
    const text = inputEl.value;
    if (!text.trim()) return;
    send({ message_type: 'text', content: text });
    inputEl.value = '';
    inputEl.style.height = 'auto';
  }
  $('sendBtn').onclick = sendText;
  inputEl.addEventListener('keydown', function (e) {
    if (e.key === 'Enter' && !e.shiftKey) { e.preventDefault(); sendText(); }
  });
  inputEl.addEventListener('input', function () {
    inputEl.style.height = 'auto';
    inputEl.style.height = Math.min(inputEl.scrollHeight, 120) + 'px';
  });

  // ---------- 表情面板 ----------
  let panelMode = null;
  function openPanel(mode) {
    panelMode = mode;
    panel.classList.add('open');
    panelTabs.innerHTML = '';
    if (mode === 'emoji') {
      renderTab('emoji', 'Emoji', true);
      window.renderEmoji(panelBody, function (e) {
        inputEl.value += e; inputEl.focus();
      });
    } else {
      window.STICKER_GROUPS.forEach((g, i) => renderTab(g.id, g.name, i === 0));
      window.loadStickers(panelBody, function (resourceId) {
        send({ message_type: 'sticker', content: resourceId });
        panel.classList.remove('open');
      });
    }
  }
  function renderTab(id, label, active) {
    const t = document.createElement('div');
    t.className = 'tab' + (active ? ' active' : '');
    t.textContent = label;
    t.onclick = function () {
      Array.from(panelTabs.children).forEach(c => c.classList.remove('active'));
      t.classList.add('active');
      if (panelMode === 'emoji') {
        window.renderEmoji(panelBody, function (e) { inputEl.value += e; inputEl.focus(); });
      } else {
        // 切换表情包分组由 loadStickers 内部处理；这里仅高亮
        window.loadStickers(panelBody, function (resourceId) {
          send({ message_type: 'sticker', content: resourceId });
          panel.classList.remove('open');
        });
      }
    };
    panelTabs.appendChild(t);
  }
  $('emojiBtn').onclick = function () {
    if (panelMode === 'emoji' && panel.classList.contains('open')) { panel.classList.remove('open'); return; }
    openPanel('emoji');
  };
  $('stickerBtn').onclick = function () {
    if (panelMode === 'sticker' && panel.classList.contains('open')) { panel.classList.remove('open'); return; }
    openPanel('sticker');
  };
  document.addEventListener('click', function (e) {
    if (!panel.contains(e.target) && e.target !== $('emojiBtn') && e.target !== $('stickerBtn')) {
      // panel.classList.remove('open');
    }
  });

  // ---------- 图片/文件上传 ----------
  $('imgBtn').onclick = function () { $('imgInput').click(); };
  $('fileBtn').onclick = function () { $('fileInput').click(); };

  $('imgInput').onchange = function () { upload(this.files[0], 'image'); this.value = ''; };
  $('fileInput').onchange = function () { upload(this.files[0], 'file'); this.value = ''; };

  async function upload(file, kind) {
    if (!file || !file.name) return;
    const fd = new FormData();
    fd.append('file', file);
    fd.append('kind', kind);
    try {
      const r = await fetch('/api/files/upload', { method: 'POST', body: fd });
      const data = await r.json();
      if (data.code !== 0) { alert('上传失败：' + (data.message || '')); return; }
      send({ message_type: kind, content: '', file_id: data.data.id });
    } catch (e) { alert('上传失败：网络错误'); }
  }

  connect();
})();
