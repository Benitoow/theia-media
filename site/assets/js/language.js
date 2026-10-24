/* Inlined before paint. Only the unqualified entry page negotiates language.
   No service, IP lookup or runtime catalogue request is involved. */
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
  if (root.dataset.entry === "true") {
    var selected = null;
    try {
      selected = match(localStorage.getItem("theia.site.language"));
    } catch (_) {}
    var preferences = navigator.languages || [navigator.language];
    for (var i = 0; !selected && i < preferences.length; i += 1)
      selected = match(preferences[i]);
    var target = new URL(
      (selected || "en") + "/",
      new URL("./", location.href),
    );
    target.search = location.search;
    target.hash = location.hash;
    location.replace(target.href);
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
