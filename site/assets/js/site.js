/* Theia public page. The page works without this file: every download link and
   every system panel is in the markup, and the scene times are plain anchors.
   What this adds is a station that shows one system at a time, a pausable
   decorative hero and a recording whose scene list follows the playhead.
   The operating system is chosen before
   the first paint by the script in the page's head, so nothing here flashes. */
(function () {
  var root = document.documentElement;

  // Language links remain real links; only an explicit choice is persisted.
  document.querySelectorAll("[data-language]").forEach(function (link) {
    link.addEventListener("click", function () {
      var target = new URL(link.href);
      target.hash = location.hash;
      target.search = location.search;
      link.href = target.href;
      try {
        localStorage.setItem("theia.site.language", link.dataset.language);
      } catch (_) {}
    });
  });

  var menus = document.querySelectorAll("[data-menu]");
  menus.forEach(function (menu) {
    menu.addEventListener("toggle", function () {
      if (menu.open)
        menus.forEach(function (other) {
          if (other !== menu) other.open = false;
        });
    });
    menu.addEventListener("keydown", function (event) {
      if (event.key === "Escape") {
        menu.open = false;
        menu.querySelector("summary").focus();
      }
    });
    menu.querySelectorAll("a").forEach(function (link) {
      link.addEventListener("click", function () {
        menu.open = false;
      });
    });
  });
  document.addEventListener("click", function (event) {
    menus.forEach(function (menu) {
      if (!menu.contains(event.target)) menu.open = false;
    });
  });

  function revealHash() {
    var id;
    try {
      id = decodeURIComponent(location.hash.slice(1));
    } catch (_) {
      return;
    }
    var target = document.getElementById(id);
    if (target && target.tagName === "DETAILS") target.open = true;
  }
  window.addEventListener("hashchange", revealHash);
  revealHash();

  var copyStatus = document.getElementById("copy-status");
  var copyTimer;
  document.querySelectorAll("[data-copy]").forEach(function (button) {
    button.addEventListener("click", async function () {
      var value = document.getElementById(button.dataset.copy).textContent;
      button.disabled = true;
      try {
        if (!navigator.clipboard || !navigator.clipboard.writeText)
          throw new Error("Clipboard unavailable");
        await navigator.clipboard.writeText(value);
        copyStatus.textContent = copyStatus.dataset.success;
      } catch (_) {
        copyStatus.textContent = copyStatus.dataset.failure;
      } finally {
        button.disabled = false;
      }
      clearTimeout(copyTimer);
      copyTimer = setTimeout(function () {
        copyStatus.textContent = "";
      }, 5000);
    });
  });

  // ---- the hero: the poster works before and without any video request ----
  var film = document.getElementById("hero-film");
  var motion = document.querySelector(".hero-motion");
  var media = document.querySelector(".hero-media");
  if (film && motion && media) {
    var calm = window.matchMedia("(prefers-reduced-motion: reduce)");
    var connection = navigator.connection;
    var visible = false;
    var manuallyPaused = false;
    var loaded = false;
    var unavailable = false;
    var fontsReady = false;
    var filmSources = film.querySelectorAll("source");
    var failedSources = [];

    function still() {
      return calm.matches || !!(connection && connection.saveData);
    }

    function motionControl() {
      motion.hidden = still() || unavailable;
      motion.setAttribute(
        "aria-label",
        film.paused ? motion.dataset.resume : motion.dataset.pause,
      );
      motion
        .querySelector("use")
        .setAttribute("href", film.paused ? "#i-play" : "#i-pause");
    }

    function mediaUnavailable() {
      unavailable = true;
      film.pause();
      media.classList.remove("is-playing");
      motionControl();
    }

    function syncMotion() {
      if (still() || unavailable) media.classList.remove("is-playing");
      if (
        still() ||
        unavailable ||
        manuallyPaused ||
        !fontsReady ||
        !visible ||
        document.hidden
      ) {
        film.pause();
      } else {
        if (!loaded) {
          filmSources.forEach(function (source) {
            source.src = source.dataset.src;
          });
          film.muted = true;
          loaded = true;
          film.load();
        }
        var started = film.play();
        if (started && started.catch)
          started.catch(function (error) {
            // Leaving the viewport while a play request loads is an ordinary pause.
            if (error.name === "AbortError") return;
            mediaUnavailable();
          });
      }
      motionControl();
    }

    film.addEventListener("playing", function () {
      if (!still() && !unavailable) media.classList.add("is-playing");
      motionControl();
    });
    film.addEventListener("pause", motionControl);
    film.addEventListener("error", mediaUnavailable);
    // Source failures need their own listeners: some browsers leave play()
    // pending when every <source> fails, without raising a video error.
    filmSources.forEach(function (source) {
      source.addEventListener("error", function () {
        if (failedSources.indexOf(source) === -1) failedSources.push(source);
        var remaining = Array.prototype.some.call(
          filmSources,
          function (candidate) {
            return (
              film.canPlayType(candidate.type) &&
              failedSources.indexOf(candidate) === -1
            );
          },
        );
        if (!remaining) mediaUnavailable();
      });
    });
    motion.addEventListener("click", function () {
      manuallyPaused = !film.paused;
      syncMotion();
    });
    calm.addEventListener("change", syncMotion);
    if (connection && connection.addEventListener)
      connection.addEventListener("change", syncMotion);
    document.addEventListener("visibilitychange", syncMotion);
    if ("IntersectionObserver" in window) {
      new IntersectionObserver(
        function (entries) {
          visible = entries[0].isIntersecting;
          syncMotion();
        },
        { threshold: 0 },
      ).observe(media);
    } else {
      visible = true;
    }
    // Resolve the critical fonts before requesting a decorative movie. Do not
    // gate this on animation frames: background windows can throttle them.
    var fontLoad = document.fonts ? document.fonts.ready : Promise.resolve();
    fontLoad.then(function () {
      fontsReady = true;
      syncMotion();
    });
    syncMotion();
  }

  // ---- the download station: one system at a time; never the processor ----
  var pills = document.querySelectorAll("[data-os-pill]");
  var live = document.getElementById("os-live");
  var names = { windows: "Windows", macos: "macOS", linux: "Linux" };

  function show(os, announce) {
    root.dataset.os = os;
    pills.forEach(function (pill) {
      pill.setAttribute("aria-pressed", String(pill.dataset.osPill === os));
    });
    if (announce && live)
      live.textContent = live.dataset.showing + " " + names[os];
  }

  pills.forEach(function (pill) {
    pill.addEventListener("click", function () {
      show(pill.dataset.osPill, true);
    });
  });
  show(root.dataset.os || "windows", false);

  // ---- the recording: scene times seek it, and the list follows it ----
  var video = document.getElementById("tour");
  var scenes = document.querySelectorAll(".scene");
  if (!video || scenes.length === 0) return;

  // The large play button belongs to the poster: it plays, then steps aside.
  var overlay = document.querySelector(".tour-play");
  if (overlay) {
    overlay.addEventListener("click", function () {
      var started = video.play();
      if (started && started.catch) started.catch(function () {});
      video.focus();
    });
    video.addEventListener("play", function () {
      overlay.hidden = true;
    });
    video.addEventListener("ended", function () {
      overlay.hidden = false;
    });
  }

  var starts = Array.prototype.map.call(scenes, function (scene) {
    return parseFloat(scene.querySelector("[data-seek]").dataset.seek);
  });

  scenes.forEach(function (scene) {
    scene
      .querySelector("[data-seek]")
      .addEventListener("click", function (event) {
        event.preventDefault();
        video.currentTime = parseFloat(this.dataset.seek);
        var started = video.play();
        if (started && started.catch) started.catch(function () {});
        video.scrollIntoView({
          block: "nearest",
          behavior: window.matchMedia("(prefers-reduced-motion: reduce)")
            .matches
            ? "instant"
            : "smooth",
        });
      });
  });

  var current = -1;
  video.addEventListener("timeupdate", function () {
    var index = 0;
    for (var i = 0; i < starts.length; i += 1) {
      if (video.currentTime >= starts[i] - 0.05) index = i;
    }
    if (index === current) return;
    current = index;
    scenes.forEach(function (scene, i) {
      if (i === index) scene.setAttribute("aria-current", "step");
      else scene.removeAttribute("aria-current");
    });
  });
})();
