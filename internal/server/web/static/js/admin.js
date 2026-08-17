(function () {
  'use strict';
  const $ = (id) => document.getElementById(id);

  function toast(msg) {
    const t = $('toast');
    t.textContent = msg;
    t.classList.add('show');
    setTimeout(() => t.classList.remove('show'), 2200);
  }
  function fmt(d) {
    if (!d) return '-';
    const dt = new Date(d);
    return dt.toLocaleString();
  }
  async function api(method, url, body) {
    const opt = { method, headers: {} };
    if (body !== undefined) {
      opt.headers['Content-Type'] = 'application/json';
      opt.body = JSON.stringify(body);
    }
    const r = await fetch(url, opt);
    return r.json();
  }

  async function loadStats() {
    const d = await api('GET', '/api/admin/stats');
    if (d.code !== 0) return;
    const s = d.data;
    $('stats').innerHTML = [
      stat(s.users_total, '用户总数'),
      stat(s.online_users, '在线用户'),
      stat(s.online_connections, '在线连接'),
      stat(s.message_retention_hours + 'h', '消息保留'),
      stat(s.session_expiration_hours + 'h', '会话有效期'),
      stat(fmtSize(s.max_file_size), '单文件上限')
    ].join('');
  }
  function stat(v, l) {
    return '<div class="stat-card"><div class="v">' + v + '</div><div class="l">' + l + '</div></div>';
  }
  function fmtSize(n) {
    if (n < 1024 * 1024) return (n / 1024).toFixed(0) + ' KB';
    return (n / 1024 / 1024).toFixed(0) + ' MB';
  }

  async function loadUsers() {
    const d = await api('GET', '/api/admin/users');
    if (d.code !== 0) { toast('加载失败'); return; }
    const tb = $('userTable');
    tb.innerHTML = '';
    d.data.forEach((u) => {
      const tr = document.createElement('tr');
      const last = u.last_login_at ? fmt(u.last_login_at) : '-';
      tr.innerHTML =
        '<td>' + esc(u.username) + '</td>' +
        '<td>' + esc(u.nickname) + '</td>' +
        '<td>' + (u.role === 'admin' ? '管理员' : '普通用户') + '</td>' +
        '<td><span class="badge ' + (u.enabled ? 'on' : 'off') + '">' + (u.enabled ? '启用' : '禁用') + '</span></td>' +
        '<td>' + fmt(u.created_at) + '</td>' +
        '<td>' + last + '</td>';
      const td = document.createElement('td');

      const toggle = btn(u.enabled ? '禁用' : '启用', '', async () => {
        await api('PUT', '/api/admin/users/' + u.id, { enabled: !u.enabled });
        loadUsers();
      });
      const reset = btn('重置密码', '', async () => {
        const p = prompt('输入新密码（≥6 位）');
        if (!p) return;
        const r = await api('PUT', '/api/admin/users/' + u.id, { password: p });
        toast(r.code === 0 ? '密码已重置' : (r.message || '失败'));
      });
      const kick = btn('强制下线', '', async () => {
        if (!confirm('强制该用户退出所有会话？')) return;
        const r = await api('POST', '/api/admin/users/' + u.id + '/reset-session');
        toast(r.code === 0 ? '已强制下线' : (r.message || '失败'));
      });
      td.appendChild(toggle); td.appendChild(reset); td.appendChild(kick);
      if (u.role !== 'admin') {
        const del = btn('删除', 'danger', async () => {
          if (!confirm('确认删除该用户？此操作不可恢复。')) return;
          const r = await api('DELETE', '/api/admin/users/' + u.id);
          toast(r.code === 0 ? '已删除' : (r.message || '失败'));
          loadUsers();
        });
        td.appendChild(del);
      }
      tr.appendChild(td);
      tb.appendChild(tr);
    });
  }
  function btn(label, cls, onclick) {
    const b = document.createElement('button');
    b.className = 'mini-btn ' + cls;
    b.textContent = label;
    b.onclick = onclick;
    return b;
  }
  function esc(s) {
    const d = document.createElement('div');
    d.textContent = s == null ? '' : s;
    return d.innerHTML;
  }

  $('createBtn').onclick = async function () {
    const username = $('cU').value.trim();
    const password = $('cP').value;
    const nickname = $('cN').value.trim();
    const enabled = $('cE').value === 'true';
    if (!username || !password) { toast('账号和密码必填'); return; }
    const r = await api('POST', '/api/admin/users', { username, password, nickname, enabled });
    if (r.code === 0) {
      toast('创建成功');
      $('cU').value = ''; $('cP').value = ''; $('cN').value = '';
      loadUsers();
    } else {
      toast('创建失败：' + (r.message || ''));
    }
  };

  loadStats();
  loadUsers();
  setInterval(loadStats, 15000);
})();
