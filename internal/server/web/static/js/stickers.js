// 表情包分组（PRD 第 9 节）。资源由服务端以 HTTP Cache + ETag 提供，浏览器自动缓存。
window.STICKER_GROUPS = [
  { id: "love", name: "爱情" },
  { id: "communication", name: "可通" },
  { id: "fun", name: "趣味" },
  { id: "mood", name: "情调" }
];

const _stickerCache = {};

// 加载表情包到容器。onPick(resourceId, imgSrc) 在点击时回调。
// resourceId 形如 "love/love_001"，仅该 ID 会被写入数据库（不存图片二进制）。
window.loadStickers = async function (container, onPick) {
  container.innerHTML = '';
  for (const g of window.STICKER_GROUPS) {
    const title = document.createElement('div');
    title.style.cssText = 'grid-column:1/-1;font-size:11px;color:#8a909c;margin:4px 2px;';
    title.textContent = g.name;
    container.appendChild(title);

    let items = _stickerCache[g.id];
    if (!items) {
      try {
        const r = await fetch('/static/stickers/' + g.id + '/manifest.json', { cache: 'force-cache' });
        if (!r.ok) continue;
        const m = await r.json();
        items = m.items || [];
        _stickerCache[g.id] = items;
      } catch (e) {
        continue;
      }
    }
    items.forEach(function (it) {
      const wrap = document.createElement('div');
      wrap.className = 'sticker-item';
      const img = document.createElement('img');
      img.loading = 'lazy';
      img.src = '/static/stickers/' + g.id + '/' + it.file;
      img.alt = it.name || g.id;
      img.onerror = function () {
        // 资源加载失败：占位，不影响其他消息。
        img.style.cssText = 'width:100%;height:60px;background:#eee;border-radius:8px;';
      };
      wrap.onclick = function () { onPick(g.id + '/' + it.id, img.src); };
      wrap.appendChild(img);
      container.appendChild(wrap);
    });
  }
};
