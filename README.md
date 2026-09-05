# Reel — terminal capability-aware content rendering pipeline

Reel renders images, animated GIFs, PDF pages, charts and Markdown to the
terminal through whatever image protocol the terminal actually supports —
kitty graphics, iTerm2 inline images, sixel or ANSI half-blocks — probing
the terminal first and degrading gracefully instead of demanding
configuration.

## Features

- **Protocol probing**: env vars, TERM heuristics, DA1/DA2 device-attribute
  arbitration on TTYs, cross-process result caching — selection is a table
  lookup at render time.
- **Degradation chain**: kitty graphics → iTerm2 → sixel → ANSI art →
  styled text placeholders, chosen per terminal, never per file.
- **Content types**: PNG/JPEG/GIF (animated where the terminal can),
  Markdown with inline images and hyperlinks, charts (bar/line/scatter,
  dual axis), PDF page previews (via poppler's `pdftoppm`).
- **Document lifecycle**: explicit placement ids, delete-before-draw on
  redraw, refit on resize — the terminal's image table never leaks.
- **TUI components and apps**: `reel read` (Markdown reader) and
  `reel gallery` (image browser) built on the same public SDK.

## Install

```
go install github.com/narcilee7/reel/cmd/reel@latest
```

Rendering images requires a terminal with graphics support (see the matrix
below); everything degrades to text placeholders otherwise.

## CLI usage

```
# render a file, auto-detecting the type (image / GIF / PDF / Markdown)
reel diagram.png
reel report.pdf          # first page preview (needs poppler)
reel notes.md

# explicit subcommands
reel img [-width N] photo.jpg     # cap width in terminal cells
reel md  [-width N] doc.md        # one-shot Markdown render

# interactive TUI apps
reel read doc.md                  # scrollable reader: lazy image loading,
                                  # n/p link cursor, enter opens in browser
reel gallery [dir]                # thumbnail grid, h/j/k/l navigate,
                                  # enter fullscreen view, esc back

# detection diagnostics (for calibration bug reports)
reel probe [-json|-template]
```

## SDK usage

```go
engine := reel.New(reel.WithAutoDetect())      // probe once, cache
content, err := reel.Load("diagram.png")       // sniffed: PNG/GIF/PDF/Markdown
if err != nil { ... }
if err := engine.Render(os.Stdout, content); err != nil { ... }
```

Long documents and interactive apps manage the terminal explicitly:

```go
doc, _ := engine.Prepare(content)   // fit, count placement ids
defer engine.DeleteImages(os.Stdout, doc.ImageIDs()...)
doc.Render(os.Stdout)               // transmit + placeholders

// Or let the Bubble Tea components do it:
pager := tui.NewPager(engine, blocks)   // lazy blocks, scroll-out deletion
```

## Protocol support matrix

| Terminal | Protocol | Static images | Animation |
|---|---|---|---|
| kitty ≥ 0.40 | kitty graphics | yes | terminal-driven (≥ 0.20) |
| ghostty | kitty graphics | yes | first frame |
| WezTerm | kitty graphics | yes | first frame |
| iTerm2 | OSC 1337 inline | yes | embedded native GIF |
| foot | sixel | yes | first frame |
| tmux ≥ 3.4 inside kitty | kitty passthrough | yes | yes |
| Apple Terminal / others | ANSI half-blocks | approximated | first frame |
| dumb / non-TTY | plain text | `[image: alt]` | `[animation: alt]` |

## Content types and degradation

- **PDF** previews rasterize the requested page through `pdftoppm`
  (poppler). Without poppler the error tells you how to install it
  (`brew install poppler` / `apt install poppler-utils`).
- **Animation** degrades per protocol: kitty ≥ 0.20 plays frames
  terminal-side, iTerm2 embeds the original GIF bytes, everything else
  shows the first frame; text-only terminals show a placeholder. The full
  matrix is pinned in [docs/PHASE3.md](docs/PHASE3.md) §1.5 and asserted
  by tests.
- Content type is detected from magic bytes first, file extension second.

## Platform support

macOS and Linux are fully featured, including TTY device-attribute
arbitration. On Windows, detection stays at the environment/TERM layers
(L3 DA queries are not implemented — ConPTY does not pass terminal
responses back cleanly); `reel probe` reports exactly which layers
contributed. Everything else — rendering, degradation, TUI apps — builds
and runs.

## Detection looks wrong?

Run `reel probe -template` and paste the output into a
[calibration issue](https://github.com/narcilee7/reel/issues/new/choose).
The raw DA1/DA2 responses let the fingerprint table be updated without
guesswork. No telemetry is collected — calibration data reaches the
project only through PRs.

## License

Apache-2.0 (see [LICENSE](LICENSE)); no source file carries a conflicting
header declaration. Dependencies (bubbletea, bubbles, goldmark, x/image,
x/sys, x/text, ...) are MIT/BSD-3-Clause, all compatible with Apache-2.0.
