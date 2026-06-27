// palette.js — the ⌘K / Ctrl-K quick-create + jump command hub. The keyboard
// counterpart to the chat panel (spec 06/08): same actions, two interaction
// styles. Pure vanilla JS, no dependency, consistent with the single-binary
// embedded-assets model.
(function () {
  var ACTIONS = [
    { label: "Overview", hint: "go", url: "/" },
    { label: "Metrics", hint: "go", url: "/metrics" },
    { label: "Sprints", hint: "go", url: "/sprints" },
    { label: "Cadence", hint: "go", url: "/cadence" },
    { label: "Logbook", hint: "go", url: "/logs" },
    { label: "ADRs", hint: "go", url: "/adrs" },
    { label: "Settings", hint: "go", url: "/settings" },
    { label: "New sprint", hint: "create", url: "/new#sprint" },
    { label: "Log today", hint: "create", url: "/new#log" },
    { label: "New post", hint: "create", url: "/new#post" },
    { label: "New ADR", hint: "create", url: "/new#adr" },
  ];

  var overlay = document.getElementById("palette");
  var input = document.getElementById("palette-input");
  var list = document.getElementById("palette-list");
  if (!overlay || !input || !list) return;

  var filtered = ACTIONS.slice();
  var active = 0;

  function render() {
    list.innerHTML = "";
    filtered.forEach(function (a, i) {
      var li = document.createElement("li");
      li.className = "palette-item" + (i === active ? " sel" : "");
      li.innerHTML =
        '<span>' + a.label + "</span>" +
        '<span class="palette-hint mono">' + a.hint + "</span>";
      li.addEventListener("click", function () { go(a); });
      list.appendChild(li);
    });
  }

  function filter() {
    var q = input.value.trim().toLowerCase();
    filtered = ACTIONS.filter(function (a) {
      return a.label.toLowerCase().indexOf(q) !== -1 || a.hint.indexOf(q) !== -1;
    });
    active = 0;
    render();
  }

  function open() {
    overlay.classList.add("show");
    overlay.setAttribute("aria-hidden", "false");
    input.value = "";
    filter();
    input.focus();
  }

  function close() {
    overlay.classList.remove("show");
    overlay.setAttribute("aria-hidden", "true");
  }

  function go(a) {
    close();
    if (a.url) window.location = a.url;
  }

  // Exposed so the topbar ⌘K button can trigger it too.
  window.scavaPalette = open;

  document.addEventListener("keydown", function (e) {
    if ((e.metaKey || e.ctrlKey) && (e.key === "k" || e.key === "K")) {
      e.preventDefault();
      overlay.classList.contains("show") ? close() : open();
      return;
    }
    if (!overlay.classList.contains("show")) return;
    if (e.key === "Escape") { close(); }
    else if (e.key === "ArrowDown") { e.preventDefault(); active = Math.min(active + 1, filtered.length - 1); render(); }
    else if (e.key === "ArrowUp") { e.preventDefault(); active = Math.max(active - 1, 0); render(); }
    else if (e.key === "Enter") { e.preventDefault(); if (filtered[active]) go(filtered[active]); }
  });

  input.addEventListener("input", filter);
  overlay.addEventListener("click", function (e) { if (e.target === overlay) close(); });
})();
