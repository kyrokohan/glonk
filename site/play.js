// glonk.dev's one script. It lets a visitor play cube3d where its recording is on the cover: the
// real engine, built for the browser by `make play` (see internal/webplay), drawn by xterm.js.
// Nothing beyond this file loads until the visitor asks for it, and without scripts the page is
// the same less the button.
"use strict";

(() => {
  const figure = document.querySelector(".lead");
  if (!figure) return;

  // The game is 120 by 40 cells of Departure Mono at 11 pixels, 7 by 14 each: 840 by 560, the
  // size of the recording it stands in for.
  const cols = 120, rows = 40, font = '11px "Departure Mono"';
  const files = {
    css: "/media/xterm/xterm.css",
    scripts: ["/media/xterm/xterm.js", "/media/xterm/addon-webgl.js", "/media/play/wasm_exec.js"],
    game: "/media/play/cube3d.wasm.gz",
  };
  const keysText = "W and S walk, A and D step aside, Q and E or the left and right arrows turn, the up and down arrows look. M switches between characters and half blocks, B the edges. Esc stops.";

  const el = (tag, props) => Object.assign(document.createElement(tag), props);
  const side = el("div", { className: "play" });
  figure.prepend(side);

  // A keyboard and room for 840 pixels: a fine pointer is the best a page can tell of the first.
  const canPlay = () =>
    typeof WebAssembly === "object" && "DecompressionStream" in window &&
    matchMedia("(any-pointer: fine) and (min-width: 868px)").matches;
  const button = el("button", { type: "button", className: "start", textContent: "Play it here" });
  const note = el("p", { className: "note", textContent: "With a keyboard and a wider screen, the game can be played here." });
  const offer = () => {
    if (figure.classList.contains("playing")) return;
    side.replaceChildren(canPlay() ? button : note);
  };
  offer();
  matchMedia("(min-width: 868px)").addEventListener("change", offer);

  const loaded = {};
  const load = (src) => (loaded[src] ??= new Promise((ok, fail) => {
    document.head.append(el("script", { src, onload: ok, onerror: () => fail(new Error("could not load " + src)) }));
  }));

  // Keys go to the engine as the kitty keyboard protocol sends them, a press and a release, so
  // two keys can be held at once. They are heard in the capture phase: xterm.js keeps a key press
  // from going further once it has it.
  const arrows = { ArrowUp: "A", ArrowDown: "B", ArrowRight: "C", ArrowLeft: "D" };
  const named = { Escape: 27, Enter: 13, Backspace: 127 };
  const sequence = (key, release) => {
    if (arrows[key]) return `\x1b[1;1:${release ? 3 : 1}${arrows[key]}`;
    const code = named[key] ?? (key.length === 1 ? key.toLowerCase().codePointAt(0) : 0);
    return code ? `\x1b[${code}${release ? ";1:3" : ""}u` : "";
  };

  async function play() {
    button.disabled = true;
    button.textContent = "Loading";
    let term;
    const box = el("div", { className: "game" });
    const stop = el("button", { type: "button", className: "start", textContent: "Stop" });
    const keys = el("p", { className: "keys", textContent: keysText });
    const held = new Set();
    const enc = new TextEncoder();
    const send = (key, release) => {
      const s = sequence(key, release);
      if (s && glonk.keys) glonk.keys(enc.encode(s));
      return s !== "";
    };
    const releaseAll = () => { for (const k of held) send(k, true); held.clear(); };

    const end = (msg) => {
      releaseAll();
      term?.dispose();
      box.remove(); keys.remove();
      figure.classList.remove("playing");
      button.disabled = false;
      button.textContent = "Play it here";
      side.replaceChildren(button);
      if (msg !== "quit") side.append(el("p", { className: "note", textContent: "The game stopped: " + msg }));
      button.focus();
    };

    try {
      if (!document.querySelector(`link[href="${files.css}"]`)) document.head.append(el("link", { rel: "stylesheet", href: files.css }));
      for (const src of files.scripts) await load(src);
      await document.fonts.load(font);
      const answer = await fetch(files.game);
      if (!answer.ok) throw new Error("could not load the game (" + answer.status + ")");
      const wasm = await new Response(answer.body.pipeThrough(new DecompressionStream("gzip"))).arrayBuffer();
      const go = new Go();
      const { instance } = await WebAssembly.instantiate(wasm, go.importObject);

      term = new Terminal({
        cols, rows, fontFamily: '"Departure Mono", monospace', fontSize: 11, lineHeight: 1,
        letterSpacing: 0, cursorBlink: false, disableStdin: true, scrollback: 0,
        theme: { background: "#000000", foreground: "#c0c0c0" },
      });
      figure.classList.add("playing");
      figure.append(box, keys);
      side.replaceChildren(stop);
      term.open(box);
      try {
        const webgl = new WebglAddon.WebglAddon();
        webgl.onContextLoss(() => webgl.dispose());
        term.loadAddon(webgl);
      } catch { /* xterm.js draws without WebGL too, with faint seams through half blocks */ }
      term.write("\x1b[?25l");

      box.addEventListener("keydown", (e) => {
        // Tab leaves the game. xterm.js would keep it for itself, so it must not see it.
        if (e.key === "Tab") { e.stopPropagation(); return; }
        if (e.metaKey || e.ctrlKey || e.altKey) return;
        if (sequence(e.key, false)) e.preventDefault();
        if (!e.repeat && !held.has(e.key) && send(e.key, false)) held.add(e.key);
      }, true);
      box.addEventListener("keyup", (e) => { if (held.delete(e.key)) send(e.key, true); }, true);
      box.addEventListener("focusout", releaseAll);
      stop.addEventListener("click", () => glonk.stop?.());

      globalThis.glonk = { cols, rows, frame: (buf, n) => term.write(buf.slice(0, n)), done: end };
      go.run(instance);
      term.focus();
    } catch (err) {
      end(err.message);
    }
  }
  button.addEventListener("click", play);
})();
