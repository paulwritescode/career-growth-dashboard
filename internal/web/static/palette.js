// palette.js — shadcn-inspired ⌘K command palette with categories, icons,
// descriptions, keyboard navigation, and search filtering.
(function () {
  // SVG icon factory (inline SVGs matching lucide-react style)
  var icons = {
    home: '<svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M3 9l9-7 9 7v11a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2z"/><polyline points="9 22 9 12 15 12 15 22"/></svg>',
    barChart: '<svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><line x1="18" y1="20" x2="18" y2="10"/><line x1="12" y1="20" x2="12" y2="4"/><line x1="6" y1="20" x2="6" y2="14"/></svg>',
    zap: '<svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><polygon points="13 2 3 14 12 14 11 22 21 10 12 10 13 2"/></svg>',
    file: '<svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M14 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V8z"/><polyline points="14 2 14 8 20 8"/></svg>',
    calendar: '<svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><rect x="3" y="4" width="18" height="18" rx="2" ry="2"/><line x1="16" y1="2" x2="16" y2="6"/><line x1="8" y1="2" x2="8" y2="6"/><line x1="3" y1="10" x2="21" y2="10"/></svg>',
    book: '<svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M4 19.5A2.5 2.5 0 0 1 6.5 17H20"/><path d="M6.5 2H20v20H6.5A2.5 2.5 0 0 1 4 19.5v-15A2.5 2.5 0 0 1 6.5 2z"/></svg>',
    plus: '<svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><circle cx="12" cy="12" r="10"/><line x1="12" y1="8" x2="12" y2="16"/><line x1="8" y1="12" x2="16" y2="12"/></svg>',
    settings: '<svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><circle cx="12" cy="12" r="3"/><path d="M19.4 15a1.65 1.65 0 0 0 .33 1.82l.06.06a2 2 0 0 1-2.83 2.83l-.06-.06a1.65 1.65 0 0 0-1.82-.33 1.65 1.65 0 0 0-1 1.51V21a2 2 0 0 1-4 0v-.09A1.65 1.65 0 0 0 9 19.4a1.65 1.65 0 0 0-1.82.33l-.06.06a2 2 0 0 1-2.83-2.83l.06-.06A1.65 1.65 0 0 0 4.68 15a1.65 1.65 0 0 0-1.51-1H3a2 2 0 0 1 0-4h.09A1.65 1.65 0 0 0 4.6 9"/></svg>',
    edit: '<svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M11 4H4a2 2 0 0 0-2 2v14a2 2 0 0 0 2 2h14a2 2 0 0 0 2-2v-7"/><path d="M18.5 2.5a2.12 2.12 0 0 1 3 3L12 15l-4 1 1-4 9.5-9.5z"/></svg>',
    refresh: '<svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><polyline points="23 4 23 10 17 10"/><path d="M20.49 15a9 9 0 1 1-2.12-9.36L23 10"/></svg>',
    copy: '<svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><rect x="9" y="9" width="13" height="13" rx="2" ry="2"/><path d="M5 15H4a2 2 0 0 1-2-2V4a2 2 0 0 1 2-2h9a2 2 0 0 1 2 2v1"/></svg>',
    maximize: '<svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M8 3H5a2 2 0 0 0-2 2v3m18 0V5a2 2 0 0 0-2-2h-3m0 18h3a2 2 0 0 0 2-2v-3M3 16v3a2 2 0 0 0 2 2h3"/></svg>',
    messageSquare: '<svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M21 15a2 2 0 0 1-2 2H7l-4 4V5a2 2 0 0 1 2-2h14a2 2 0 0 1 2 2z"/></svg>'
  };

  // Command definitions
  var COMMANDS = [
    // Navigate
    { id:'nav-overview', title:'Overview', desc:'Dashboard overview', cat:'navigate', icon:'home', url:'/', shortcut:'Alt+1' },
    { id:'nav-metrics', title:'Metrics', desc:'Cadence and shipping metrics', cat:'navigate', icon:'barChart', url:'/metrics', shortcut:'Alt+2' },
    { id:'nav-sprints', title:'Sprints', desc:'Monthly skill sprints', cat:'navigate', icon:'zap', url:'/sprints', shortcut:'Alt+3' },
    { id:'nav-cadence', title:'Cadence', desc:'Content publishing cadence', cat:'navigate', icon:'calendar', url:'/cadence' },
    { id:'nav-logbook', title:'Logbook', desc:'Build logs and career events', cat:'navigate', icon:'book', url:'/logs' },
    { id:'nav-adrs', title:'ADRs', desc:'Architecture decision records', cat:'navigate', icon:'file', url:'/adrs' },
    { id:'nav-settings', title:'Settings', desc:'App configuration', cat:'navigate', icon:'settings', url:'/settings' },
    // Create
    { id:'create-sprint', title:'New Sprint', desc:'Start a new monthly skill sprint', cat:'create', icon:'zap', url:'/new#sprint' },
    { id:'create-log', title:'Log Today', desc:'Record today\'s build log', cat:'create', icon:'edit', url:'/new#log' },
    { id:'create-post', title:'New Post', desc:'Create a daily or recap post', cat:'create', icon:'edit', url:'/new#post' },
    { id:'create-adr', title:'New ADR', desc:'Record an architecture decision', cat:'create', icon:'file', url:'/new#adr' },
    // Actions
    { id:'action-refresh', title:'Refresh Page', desc:'Reload the current page', cat:'action', icon:'refresh', action:function(){ location.reload(); }, shortcut:'Ctrl+R' },
    { id:'action-copy-url', title:'Copy URL', desc:'Copy current page URL to clipboard', cat:'action', icon:'copy', action:function(){ navigator.clipboard.writeText(location.href); } },
    { id:'action-fullscreen', title:'Toggle Fullscreen', desc:'Enter or exit fullscreen mode', cat:'action', icon:'maximize', action:function(){ document.fullscreenElement ? document.exitFullscreen() : document.documentElement.requestFullscreen(); }, shortcut:'F11' },
    { id:'action-chat', title:'Open Chat', desc:'Open the AI assistant chat widget', cat:'action', icon:'messageSquare', action:function(){ var fcw = document.getElementById('fcw'); if(fcw) fcw.classList.add('fcw-open'); } }
  ];

  var overlay = document.getElementById('palette');
  var panel = document.getElementById('palette-panel');
  var input = document.getElementById('palette-input');
  var list = document.getElementById('palette-list');
  var catContainer = document.getElementById('palette-categories');
  if (!overlay || !input || !list) return;

  var filtered = COMMANDS.slice();
  var active = 0;
  var activeCat = 'all';

  function getFiltered() {
    var q = input.value.trim().toLowerCase();
    return COMMANDS.filter(function(cmd) {
      if (activeCat !== 'all' && cmd.cat !== activeCat) return false;
      if (!q) return true;
      return cmd.title.toLowerCase().indexOf(q) !== -1 ||
             cmd.desc.toLowerCase().indexOf(q) !== -1 ||
             cmd.cat.indexOf(q) !== -1;
    });
  }

  function render() {
    filtered = getFiltered();
    active = Math.min(active, Math.max(filtered.length - 1, 0));
    list.innerHTML = '';

    if (filtered.length === 0) {
      list.innerHTML = '<div class="palette-empty">' +
        '<svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><circle cx="11" cy="11" r="8"/><line x1="21" y1="21" x2="16.65" y2="16.65"/></svg>' +
        '<p>No commands found</p>' +
        '<button onclick="document.getElementById(\'palette-input\').value=\'\';document.getElementById(\'palette-input\').dispatchEvent(new Event(\'input\'))">Clear search</button></div>';
      return;
    }

    // Group by category
    var groups = {};
    filtered.forEach(function(cmd) {
      if (!groups[cmd.cat]) groups[cmd.cat] = [];
      groups[cmd.cat].push(cmd);
    });

    var globalIdx = 0;
    var catLabels = { navigate:'Navigate', create:'Create', action:'Actions' };

    Object.keys(groups).forEach(function(cat) {
      // Group header
      var header = document.createElement('li');
      header.className = 'palette-group';
      header.innerHTML = '<span>' + (catLabels[cat]||cat) + '</span><span class="palette-group-count">' + groups[cat].length + '</span>';
      list.appendChild(header);

      groups[cat].forEach(function(cmd) {
        var li = document.createElement('li');
        li.className = 'palette-item' + (globalIdx === active ? ' sel' : '');
        li.setAttribute('data-idx', globalIdx);

        var iconHtml = icons[cmd.icon] || '';
        var shortcutHtml = cmd.shortcut ? '<kbd class="palette-kbd">' + cmd.shortcut + '</kbd>' : '';

        li.innerHTML =
          '<div class="palette-item-left">' +
            '<div class="palette-item-icon">' + iconHtml + '</div>' +
            '<div class="palette-item-text">' +
              '<span class="palette-item-title">' + cmd.title + '</span>' +
              '<span class="palette-item-desc">' + cmd.desc + '</span>' +
            '</div>' +
          '</div>' +
          '<div class="palette-item-right">' +
            '<span class="palette-item-cat">' + (catLabels[cmd.cat]||cmd.cat) + '</span>' +
            shortcutHtml +
          '</div>';

        li.addEventListener('click', function() { execute(cmd); });
        li.addEventListener('mouseenter', function() {
          active = parseInt(this.getAttribute('data-idx'));
          highlightActive();
        });
        list.appendChild(li);
        globalIdx++;
      });
    });
  }

  function highlightActive() {
    var items = list.querySelectorAll('.palette-item');
    items.forEach(function(el, i) {
      el.classList.toggle('sel', parseInt(el.getAttribute('data-idx')) === active);
    });
    // Scroll into view
    var sel = list.querySelector('.palette-item.sel');
    if (sel) sel.scrollIntoView({ block:'nearest', behavior:'smooth' });
  }

  function execute(cmd) {
    close();
    if (cmd.action) { cmd.action(); }
    else if (cmd.url) { window.location = cmd.url; }
  }

  function open() {
    overlay.classList.add('show');
    overlay.setAttribute('aria-hidden', 'false');
    input.value = '';
    activeCat = 'all';
    updateCatPills();
    active = 0;
    render();
    input.focus();
  }

  function close() {
    overlay.classList.remove('show');
    overlay.setAttribute('aria-hidden', 'true');
  }

  function updateCatPills() {
    var btns = catContainer.querySelectorAll('.palette-cat');
    btns.forEach(function(btn) {
      btn.classList.toggle('active', btn.getAttribute('data-cat') === activeCat);
    });
  }

  // Category pill clicks
  catContainer.addEventListener('click', function(e) {
    var btn = e.target.closest('.palette-cat');
    if (!btn) return;
    activeCat = btn.getAttribute('data-cat');
    updateCatPills();
    active = 0;
    render();
    input.focus();
  });

  // Expose for topbar button
  window.scavaPalette = open;

  // Keyboard handling
  document.addEventListener('keydown', function(e) {
    if ((e.metaKey || e.ctrlKey) && (e.key === 'k' || e.key === 'K')) {
      e.preventDefault();
      overlay.classList.contains('show') ? close() : open();
      return;
    }
    if (!overlay.classList.contains('show')) return;

    if (e.key === 'Escape') { e.preventDefault(); close(); }
    else if (e.key === 'ArrowDown') {
      e.preventDefault();
      active = Math.min(active + 1, filtered.length - 1);
      highlightActive();
    }
    else if (e.key === 'ArrowUp') {
      e.preventDefault();
      active = Math.max(active - 1, 0);
      highlightActive();
    }
    else if (e.key === 'Enter') {
      e.preventDefault();
      if (filtered[active]) execute(filtered[active]);
    }
  });

  input.addEventListener('input', function() { active = 0; render(); });
  overlay.addEventListener('click', function(e) { if (e.target === overlay) close(); });
})();
