/* Inlined before paint. Only the unqualified entry page negotiates language.
   No service, IP lookup or runtime catalogue request is involved. An address
   from the single-page site keeps working: its section names the page that holds
   it now, and the visitor is sent there once, with the query and the section. */
(function () {
  var root = document.documentElement;
  var supported = root.dataset.locales.split(",");
  function match(value) {
    if (typeof value !== "string") return null;
    var language = value.toLowerCase().replace(/_/g, "-");
    if (supported.indexOf(language) !== -1) return language;
    var base = language.split("-")[0];
    if (base === "zh") return "zh-hans";
    if (base === "pt") return "pt-br";
    if (base === "in") return "id";
    return supported.indexOf(base) !== -1 ? base : null;
  }

  // Sections that used to live on the one page and now have a page of their own.
  // #discover, #demo and #help stay on the home page, so they are not listed.
  var moved = {
    download: "download",
    new: "changelog",
    changelog: "changelog",
    v5: "changelog",
    faq: "questions",
    ledger: "questions",
    limits: "questions",
    privacy: "questions",
    licence: "about",
  };
  function pageOf(hash) {
    var id;
    try {
      id = decodeURIComponent(String(hash || "").replace(/^#/, ""));
    } catch (_) {
      return null;
    }
    if (Object.prototype.hasOwnProperty.call(moved, id)) return moved[id];
    return /^faq-/.test(id) ? "questions" : null;
  }

  if (root.dataset.entry === "true") {
    var selected = null;
    try {
      selected = match(localStorage.getItem("theia.site.language"));
    } catch (_) {}
    var preferences = navigator.languages || [navigator.language];
    for (var i = 0; !selected && i < preferences.length; i += 1)
      selected = match(preferences[i]);
    var page = pageOf(location.hash);
    var target = new URL(
      (selected || "en") + "/" + (page ? page + "/" : ""),
      new URL("./", location.href),
    );
    target.search = location.search;
    target.hash = location.hash;
    location.replace(target.href);
  } else {
    var elsewhere = pageOf(location.hash);
    if (elsewhere && elsewhere !== (root.dataset.page || "home")) {
      var destination = new URL(
        (root.dataset.home || "./") + elsewhere + "/",
        location.href,
      );
      destination.search = location.search;
      destination.hash = location.hash;
      location.replace(destination.href);
    }
  }

  var hints = navigator.userAgentData;
  var agent = navigator.userAgent || "";
  var platform = (hints && hints.platform) || navigator.platform || "";
  var phone =
    !!(hints && hints.mobile) ||
    /Android|iPhone|iPad|iPod|Mobile/i.test(agent) ||
    (/Macintosh/.test(agent) && navigator.maxTouchPoints > 1);
  var os = "windows";
  if (!phone) {
    if (/^Mac/i.test(platform) || /Macintosh/.test(agent)) os = "macos";
    else if (/Linux|CrOS|X11/i.test(platform + " " + agent)) os = "linux";
  }
  root.dataset.os = os;
  if (phone) root.dataset.mobile = "";
  root.classList.add("js");
})();
