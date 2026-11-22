// glonk.dev's one script. It lets a visitor play cube3d where its recording is on the cover: the
// real engine, built for the browser by `make play` and run by the glonk.dev/web package, drawn by
// xterm.js. This file is the cover's part: the offer, the button and the keys. Starting the game and
// wiring the keys, a paste and the pointer to it is glonk.js, from the web package, which any site
// can copy. Nothing beyond this file loads until the visitor asks for it, and without scripts the
// page is the same less the button.
"use strict";

(() => {
  const figure = document.querySelector(".lead");
  if (!figure) return;

  // The game is 120 by 40 cells of Departure Mono at 11 pixels, 7 by 14 each: 840 by 560, the
  // size of the recording it stands in for.
  const cols = 120, rows = 40, font = '11px "Departure Mono"';
  const files = {
    css: "/media/xterm/xterm.css",
    scripts: ["/media/xterm/xterm.js", "/media/xterm/addon-webgl.js", "/media/play/wasm_exec.js", "/media/play/glonk.js"],
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

  async function play() {
    button.disabled = true;
    button.textContent = "Loading";
    const box = el("div", { className: "game" });
    const stop = el("button", { type: "button", className: "start", textContent: "Stop" });
    const keys = el("p", { className: "keys", textContent: keysText });

    const end = (msg) => {
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
      figure.classList.add("playing");
      figure.append(box, keys);
      const game = await glonkPlay(box, {
        game: files.game, cols, rows,
        terminal: { fontFamily: '"Departure Mono", monospace', fontSize: 11, theme: { background: "#000000", foreground: "#c0c0c0" } },
      });
      side.replaceChildren(stop);
      stop.addEventListener("click", game.stop);
      end(await game.done);
    } catch (err) {
      end(err.message);
    }
  }
  button.addEventListener("click", play);
})();
