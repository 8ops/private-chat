// 内置 Emoji 数据（系统原生 Unicode，使用终端字体渲染，无需额外资源下载）。
window.EMOJI_DATA = {
  "笑脸": ["😀","😁","😂","🤣","😊","😍","😘","😎","🤔","😅","😉","🙂","🙃","😴","😇","🤩","😋","😜","🤗","😏"],
  "情感": ["😢","😭","😡","😱","😨","🥺","😳","🤯","😤","😬","🤮","🤡","😈","👿","💀","🤯","😵","😷","🤒","🥳"],
  "手势": ["👍","👎","👌","✌️","🤞","🤙","👏","🙌","🙏","💪","👋","🤝","✊","👊","🫶","👀","🫡","🤜","🤛","🖕"],
  "人物": ["👶","🧒","👦","👧","👨","👩","🧑","👴","👵","🧔","👮","👷","💂","🕵️","👰","🤰","🧑‍💻","🧑‍🚀","🦸","🧚"],
  "动物": ["🐶","🐱","🐭","🐹","🐰","🦊","🐻","🐼","🐨","🐯","🦁","🐮","🐷","🐸","🐵","🐔","🐧","🦄","🐝","🦋"],
  "自然": ["🌞","🌝","⭐","🌟","🔥","💧","🌊","🌈","🌸","🌹","🌻","🍀","🌿","🍂","❄️","⚡","🌍","🌙","☀️","🌈"],
  "食物": ["🍎","🍊","🍋","🍉","🍇","🍓","🍒","🍑","🥭","🍍","🍔","🍟","🍕","🌭","🍜","🍣","🍰","🍩","☕","🍺"],
  "物品": ["💡","📱","💻","⌨️","🖥️","📷","🎮","🎵","📚","✏️","📌","🔑","🔒","🎁","💰","⏰","🚀","⚽","🏀","🎯"],
  "符号": ["❤️","🧡","💛","💚","💙","💜","🖤","💔","✅","❌","⭐","🔔","🚫","💯","➕","➖","❓","❗","💢","🔥"]
};

// 渲染 Emoji 选择器到容器，onPick(emoji) 在点击时回调。
window.renderEmoji = function (container, onPick) {
  container.innerHTML = '';
  const cats = Object.keys(window.EMOJI_DATA);
  cats.forEach(function (cat) {
    const title = document.createElement('div');
    title.style.cssText = 'grid-column:1/-1;font-size:11px;color:#8a909c;margin:4px 2px;';
    title.textContent = cat;
    container.appendChild(title);
    window.EMOJI_DATA[cat].forEach(function (e) {
      const item = document.createElement('div');
      item.className = 'emoji-item';
      item.textContent = e;
      item.onclick = function () { onPick(e); };
      container.appendChild(item);
    });
  });
};
