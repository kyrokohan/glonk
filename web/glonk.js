// glonk.js starts a glonk game, built for the browser with the glonk.dev/web package, in an element of
// a page, and wires the keys, a paste, the pointer and the wheel to it. Copy it from glonk's module, as
// wasm_exec.js comes from Go:
//
//   cp "$(go list -m -f '{{.Dir}}' glonk.dev)/web/glonk.js" .
//
// It needs, already loaded: xterm.js (Terminal), its WebGL addon (WebglAddon) for half blocks without
// seams, and wasm_exec.js (Go) from the Go that built the game. One game runs on a page at a time.
"use strict";

/**
 * Starts the game at options.game in box, an element on the page, drawn by xterm.js in cols by rows
 * cells. options.game is the URL of the build, .wasm, or .wasm.gz gzipped, which is unpacked here.
 * options.terminal is passed on to xterm.js's Terminal, for the font and the colors.
 *
 * It resolves once the game runs, to { done, stop }: done is a promise of how the game ended, "quit"
 * or what went wrong, and stop ends it. It rejects where the game cannot be loaded or started.
 */
globalThis.glonkPlay = async function glonkPlay(box, { game, cols = 80, rows = 24, terminal = {} }) {
  if (globalThis.glonk) throw new Error("a glonk game is already running on this page");

  const answer = await fetch(game);
  if (!answer.ok) throw new Error(`could not load the game (${answer.status})`);
  const body = /\.gz$/.test(new URL(game, location.href).pathname)
    ? answer.body.pipeThrough(new DecompressionStream("gzip")) : answer.body;
  const go = new Go();
  const { instance } = await WebAssembly.instantiate(await new Response(body).arrayBuffer(), go.importObject);

  const term = new Terminal({
    cursorBlink: false, disableStdin: true, scrollback: 0, lineHeight: 1, letterSpacing: 0,
    ...terminal, cols, rows,
  });
  term.open(box);
  try {
    const webgl = new WebglAddon.WebglAddon();
    webgl.onContextLoss(() => webgl.dispose());
    term.loadAddon(webgl);
    // shortcut: addon-webgl 0.19.0 draws wrong cells once its atlas merges pages (xterm.js #5883, glonk
    // #164), so a fresh atlas starts long before that; delete once a stable xterm.js has the fix.
    let pages = 1;
    webgl.onChangeTextureAtlas(() => { pages = 1; });
    webgl.onAddTextureAtlasCanvas(() => { pages++; });
    webgl.onRemoveTextureAtlasCanvas(() => { pages--; });
    term.onRender(() => {
      if (pages < 4) return;
      // An atlas option changed makes a new atlas, and changed back, a fresh one like the first.
      term.options.drawBoldTextInBrightColors = !term.options.drawBoldTextInBrightColors;
      term.options.drawBoldTextInBrightColors = !term.options.drawBoldTextInBrightColors;
    });
  } catch { /* xterm.js draws without WebGL too, with faint seams through half blocks */ }
  term.write("\x1b[?25l");

  // What the page tells the game, each heard where xterm.js would not stop it: in the capture phase.
  const listeners = [];
  const on = (type, fn, options = true) => {
    box.addEventListener(type, fn, options);
    listeners.push([type, fn, options]);
  };
  const call = (name, ...args) => globalThis.glonk?.[name]?.(...args);
  const mods = (e) => (e.shiftKey ? 1 : 0) | (e.altKey ? 2 : 0) | (e.ctrlKey ? 4 : 0) | (e.metaKey ? 8 : 0);
  const mine = () => box.contains(document.activeElement);

  // Tab leaves the game and Command is the browser's: neither is the game's, nor shown to xterm.js.
  const browsers = (e) => e.key === "Tab" || e.metaKey;
  on("keydown", (e) => {
    if (browsers(e)) { e.stopPropagation(); return; }
    e.preventDefault();
    call("key", e.key, e.code, mods(e), e.repeat ? 2 : 1);
  });
  on("keyup", (e) => {
    if (browsers(e)) return;
    call("key", e.key, e.code, mods(e), 3);
  });
  on("paste", (e) => {
    const text = e.clipboardData?.getData("text");
    e.preventDefault();
    if (text) call("paste", text);
  });
  on("focusin", () => call("focus", true));
  on("focusout", () => call("focus", false));

  // The pointer, in the cell it is over. A button held in a move is the left, the middle or the right.
  const cell = (e) => {
    const r = (box.querySelector(".xterm-screen") ?? box).getBoundingClientRect();
    return [Math.floor((e.clientX - r.left) * cols / r.width), Math.floor((e.clientY - r.top) * rows / r.height)];
  };
  const heldButton = (b) => (b & 1 ? 0 : b & 4 ? 1 : b & 2 ? 2 : -1);
  on("pointermove", (e) => call("pointer", "move", heldButton(e.buttons), ...cell(e)));
  on("pointerdown", (e) => call("pointer", "down", e.button, ...cell(e)));
  on("pointerup", (e) => call("pointer", "up", e.button, ...cell(e)));
  // The wheel and the right button are the game's only while it has the keyboard, so that a visitor
  // can still scroll the page past it.
  on("wheel", (e) => {
    if (!mine()) return;
    e.preventDefault();
    const [col, row] = cell(e);
    if (e.deltaY) call("pointer", "wheel", e.deltaY < 0 ? 0 : 1, col, row);
    if (e.deltaX) call("pointer", "wheel", e.deltaX < 0 ? 2 : 3, col, row);
  }, { capture: true, passive: false });
  on("contextmenu", (e) => { if (mine()) e.preventDefault(); });

  let finish;
  const done = new Promise((resolve) => { finish = resolve; });
  globalThis.glonk = {
    cols, rows,
    // A hidden tab draws nothing. Each frame comes whole, so the first one after it shows again
    // repaints all of it.
    frame: (buf, n) => { if (!document.hidden) term.write(buf.slice(0, n)); },
    done: (message) => {
      for (const [type, fn, options] of listeners) box.removeEventListener(type, fn, options);
      term.dispose();
      delete globalThis.glonk;
      finish(message);
    },
  };
  go.run(instance); // never settles: the game's Run does not return
  term.focus();
  return { done, stop: () => call("stop") };
};
