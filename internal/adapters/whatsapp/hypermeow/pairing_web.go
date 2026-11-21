package hypermeow

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"rsc.io/qr"
)

// webAssetDirectory holds the static images the pairing page uses, relative to
// the process working directory. JoyX keeps its own copy so the page never
// depends on an external picture library.
const webAssetDirectory = "web"

// razzLink is the Instagram credit box at the foot of the page.
const razzLink = "https://instagram.com/razaraheeshs"

// githubLink is the GitHub credit box beside it.
const githubLink = "https://github.com/EllMaLuvv"

// WebPairingSink shows the pairing QR in a browser page, so the person
// linking the phone can scan it instead of reading a terminal full-block
// drawing. It serves the current payload only, on the address it is given,
// and refreshes itself until WhatsApp accepts the link.
type WebPairingSink struct {
	// Address is where the page listens, for example 127.0.0.1:3000.
	Address string

	mu      sync.Mutex
	code    string
	expires time.Time
	history HistorySource
	server  *http.Server
	serving bool
}

// HistoryMessage is one of Joy's own messages shown in the page's history panel.
type HistoryMessage struct {
	ChatID string    `json:"chatId"`
	Text   string    `json:"text"`
	At     time.Time `json:"at"`
}

// HistorySource supplies Joy's recent messages. It lives at the application
// layer, which owns storage; a nil source simply leaves the panel empty.
type HistorySource interface {
	RecentMessages(ctx context.Context, since time.Time, limit int) ([]HistoryMessage, error)
}

// historyLimit caps how many lines the panel asks for.
const historyLimit = 40

// historyWindow is how far back the panel looks. WhatsApp itself keeps a
// conversation around far longer; this is only what the page is meant to show.
const historyWindow = 7 * 24 * time.Hour

// SetHistorySource attaches the message history the page shows. Called once
// during composition, after the store exists.
func (sink *WebPairingSink) SetHistorySource(source HistorySource) {
	sink.mu.Lock()
	defer sink.mu.Unlock()
	sink.history = source
}

// Start begins serving the page before any pairing code arrives, so the URL is
// always open. Without a code the page explains that no pairing is active.
func (sink *WebPairingSink) Start() {
	if sink.Address == "" {
		sink.Address = "127.0.0.1:3001"
	}
	sink.mu.Lock()
	defer sink.mu.Unlock()
	if sink.serving {
		return
	}
	sink.serving = true
	go sink.serve()
}

// ShowPairingCode stores the payload and makes sure the page is being served.
func (sink *WebPairingSink) ShowPairingCode(code string, validFor time.Duration) error {
	if sink == nil {
		return errors.New("pairing output is unavailable")
	}
	if code == "" || len(code) > 2048 || validFor <= 0 {
		return errors.New("pairing payload or lifetime is invalid")
	}
	if sink.Address == "" {
		sink.Address = "127.0.0.1:3001"
	}
	sink.Start()
	sink.mu.Lock()
	sink.code, sink.expires = code, time.Now().Add(validFor)
	sink.mu.Unlock()
	return nil
}

func (sink *WebPairingSink) serve() {
	listener, err := net.Listen("tcp", sink.Address)
	if err != nil {
		fmt.Fprintf(os.Stderr, "pairing page could not listen on %s: %v\n", sink.Address, err)
		return
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/", sink.page)
	mux.HandleFunc("/qr.png", sink.image)
	mux.HandleFunc("/logo.jpg", sink.asset("logo.jpg"))
	mux.HandleFunc("/background.jpg", sink.asset("background.jpg"))
	mux.HandleFunc("/history.json", sink.historyJSON)
	sink.server = &http.Server{Handler: mux, ReadHeaderTimeout: 5 * time.Second}
	fmt.Fprintf(os.Stdout, "\nWhatsApp pairing QR page: http://%s\n", listener.Addr().String())
	_ = sink.server.Serve(listener)
}

func (sink *WebPairingSink) page(writer http.ResponseWriter, request *http.Request) {
	if request.URL.Path != "/" {
		http.NotFound(writer, request)
		return
	}
	sink.mu.Lock()
	code := sink.code
	expires := sink.expires
	sink.mu.Unlock()
	writer.Header().Set("Content-Type", "text/html; charset=utf-8")
	writer.Header().Set("Cache-Control", "no-store")
	if code != "" {
		fmt.Fprint(writer, strings.ReplaceAll(pairingPage, "{{countdown}}", time.Until(expires).Round(time.Second).String()))
		return
	}
	_, _ = fmt.Fprint(writer, pairingPageWaiting)
}

func (sink *WebPairingSink) image(writer http.ResponseWriter, request *http.Request) {
	sink.mu.Lock()
	code := sink.code
	sink.mu.Unlock()
	if code == "" {
		http.Error(writer, "no pairing code yet", http.StatusServiceUnavailable)
		return
	}
	encoded, err := qr.Encode(code, qr.L)
	if err != nil {
		http.Error(writer, "could not draw the QR code", http.StatusInternalServerError)
		return
	}
	writer.Header().Set("Content-Type", "image/png")
	writer.Header().Set("Cache-Control", "no-store")
	_, _ = writer.Write(encoded.PNG())
}

// historyJSON answers the page's message-history panel: Joy's own messages from
// the last seven days, newest first.
func (sink *WebPairingSink) historyJSON(writer http.ResponseWriter, request *http.Request) {
	sink.mu.Lock()
	source := sink.history
	sink.mu.Unlock()
	writer.Header().Set("Content-Type", "application/json; charset=utf-8")
	writer.Header().Set("Cache-Control", "no-store")
	messages := []HistoryMessage{}
	if source != nil {
		recent, err := source.RecentMessages(request.Context(), time.Now().Add(-historyWindow), historyLimit)
		if err != nil {
			writer.WriteHeader(http.StatusServiceUnavailable)
		} else if len(recent) > 0 {
			messages = recent
		}
	}
	encoded, err := json.Marshal(struct {
		Messages []HistoryMessage `json:"messages"`
	}{Messages: messages})
	if err != nil {
		http.Error(writer, "could not encode history", http.StatusInternalServerError)
		return
	}
	_, _ = writer.Write(encoded)
}

// asset serves one static image from the web directory. A missing file is a
// 404, and the page falls back to its plain colours.
func (sink *WebPairingSink) asset(name string) http.HandlerFunc {
	return func(writer http.ResponseWriter, request *http.Request) {
		data, err := os.ReadFile(filepath.Join(webAssetDirectory, name))
		if err != nil {
			http.Error(writer, "image is not installed", http.StatusNotFound)
			return
		}
		writer.Header().Set("Content-Type", "image/jpeg")
		writer.Header().Set("Cache-Control", "public, max-age=300")
		_, _ = writer.Write(data)
	}
}

// pairingStyles follows design-system/joyx/MASTER.md: dark OLED surface, slate
// text, one green accent, a 4/8 spacing rhythm and 44px touch targets. Motion
// stays on transform/opacity and is switched off under reduced motion.
const pairingStyles = `
:root {
  --color-background: #020617;
  --color-foreground: #f8fafc;
  --color-muted: #94a3b8;
  --color-border: #334155;
  --color-accent: #16a34a;
  --color-on-accent: #0f172a;
  --space-2: 8px; --space-3: 12px; --space-4: 16px;
  --space-6: 24px; --space-8: 32px;
  --radius-md: 12px; --radius-lg: 16px;
}
* { box-sizing: border-box }
body {
  margin: 0; min-height: 100vh; display: flex; align-items: center; justify-content: center;
  padding: var(--space-4); color: var(--color-foreground);
  background-color: var(--color-background);
  background-image:
    radial-gradient(60vw 60vw at 12% 8%, rgba(22, 163, 74, 0.22), transparent 60%),
    radial-gradient(55vw 55vw at 88% 92%, rgba(51, 65, 85, 0.30), transparent 62%),
    linear-gradient(rgba(2, 6, 23, 0.84), rgba(2, 6, 23, 0.84)),
    url("/background.jpg");
  background-size: cover; background-position: center; background-attachment: fixed;
  font-family: "Fira Sans", system-ui, -apple-system, "Segoe UI", sans-serif;
  font-size: 16px; line-height: 1.5;
}
.brand {
  position: fixed; top: var(--space-4); left: var(--space-4); z-index: 2;
  display: inline-flex; align-items: center; gap: var(--space-3);
  padding: var(--space-2) var(--space-4) var(--space-2) var(--space-2);
  background: rgba(2, 6, 23, 0.72); border: 1px solid var(--color-border);
  border-radius: 999px;
}
.brand img {
  width: 40px; height: 40px; border-radius: 50%; object-fit: cover;
  display: block; background: #fff;
}
.brand .name { font-size: 1rem; font-weight: 600; letter-spacing: .01em }
.brand .version {
  font-size: .75rem; font-weight: 600; color: var(--color-on-accent);
  background: var(--color-accent); border-radius: 999px; padding: 2px var(--space-2);
}
.card {
  position: relative; width: min(100%, 26rem); padding: var(--space-8); text-align: center;
  background: rgba(14, 18, 35, 0.88); border: 1px solid var(--color-border);
  border-radius: var(--radius-lg);
  transition: transform 200ms ease-out, border-color 200ms ease-out;
}
.card:hover { transform: translateY(-3px); border-color: var(--color-accent) }
h1 { margin: 0 0 var(--space-2); font-size: 1.5rem; line-height: 1.3; font-weight: 600 }
p { margin: 0 0 var(--space-4); color: var(--color-muted) }
.qr {
  display: block; width: min(100%, 21rem); aspect-ratio: 1 / 1; object-fit: contain;
  margin: 0 auto var(--space-4); padding: var(--space-3);
  background: #ffffff; border-radius: var(--radius-md);
}
.status { color: var(--color-foreground) }
.actions { display: flex; gap: var(--space-3); justify-content: center; flex-wrap: wrap }
a.button {
  display: inline-flex; align-items: center; justify-content: center;
  min-height: 44px; padding: 0 var(--space-4); border-radius: var(--radius-md);
  background: var(--color-accent); color: var(--color-on-accent);
  font-weight: 600; text-decoration: none; cursor: pointer;
  transition: filter 150ms ease-out, transform 150ms ease-out;
}
a.button:hover { filter: brightness(1.08); transform: translateY(-1px) }
a.button:focus-visible { outline: 3px solid #ffffff; outline-offset: 2px }
a.plain { color: var(--color-foreground); min-height: 44px; display: inline-flex; align-items: center }
a.plain:focus-visible { outline: 3px solid #ffffff; outline-offset: 2px; border-radius: 4px }
.credits {
  position: fixed; bottom: var(--space-4); left: 50%; transform: translateX(-50%); z-index: 2;
  display: flex; gap: var(--space-3); flex-wrap: wrap; justify-content: center;
}
.credit {
  display: inline-flex; align-items: center; justify-content: center; gap: var(--space-2);
  min-width: 7rem; min-height: 44px; padding: var(--space-2) var(--space-6);
  background: rgba(2, 6, 23, 0.78); border: 1px solid var(--color-border);
  border-radius: 999px; color: var(--color-foreground);
  font-weight: 600; text-decoration: none;
  transition: transform 150ms ease-out, border-color 150ms ease-out;
}
.credit:hover { transform: translateY(-2px); border-color: var(--color-accent) }
.credit:focus-visible { outline: 3px solid #ffffff; outline-offset: 2px }
.credit svg { width: 18px; height: 18px; fill: currentColor; flex: none }
@media (prefers-reduced-motion: reduce) {
  * { transition: none !important; animation: none !important }
}
/* Joy's message history: a compact panel in the top-right corner. */
.history {
  position: fixed; top: 16px; right: 16px; z-index: 5;
  width: 288px; max-height: 38vh;
  display: flex; flex-direction: column; gap: 8px;
  padding: 12px; overflow: hidden; box-sizing: border-box;
  background: rgba(15, 23, 42, 0.72); backdrop-filter: blur(12px);
  border: 1px solid #334155; border-radius: 16px;
}
.history h2 {
  margin: 0; font-size: 0.8rem; letter-spacing: 0.04em; text-transform: uppercase;
  color: var(--color-muted); font-weight: 600;
}
.history ul {
  list-style: none; margin: 0; padding: 0; overflow-y: auto;
  display: flex; flex-direction: column; gap: var(--space-2);
}
.history li { margin: 0; font-size: 0.82rem; line-height: 1.35; color: var(--color-foreground) }
.history time { display: block; font-size: 0.7rem; color: var(--color-muted) }
.history .empty { color: var(--color-muted); font-size: 0.82rem }
@media (max-width: 640px), (max-height: 559px), (max-width: 900px) {
  /* Too little room beside the card: stack the panel under it instead of
     overlapping, so the page stays readable on a laptop or a phone. */
  body { display: flex; flex-direction: column; align-items: center }
  .card { order: 1 }
  .history {
    order: 2; position: static; width: min(100%, 26rem); max-height: 14rem;
    margin: var(--space-6) 0 0;
  }
  .credits { order: 3 }
}
`

// historyBlock is the message-history panel shown on both pages.
const historyBlock = `<aside class="history" id="history" aria-label="Riwayat pesan Joy">
<h2>Riwayat Joy &middot; 7 hari</h2>
<ul id="historyList"><li class="empty">Memuat&hellip;</li></ul>
` + historyScript + `</aside>
`

// historyScript fills the panel from /history.json, newest first, and refreshes
// it every half minute. Text goes in through textContent, so a quote or a link
// Joy sent can never be read as markup.
const historyScript = `<script>
(function () {
  var list = document.getElementById("historyList");
  function render(messages) {
    list.textContent = "";
    if (!messages.length) {
      var empty = document.createElement("li");
      empty.className = "empty";
      empty.textContent = "Belum ada pesan Joy dalam 7 hari terakhir.";
      list.appendChild(empty);
      return;
    }
    messages.forEach(function (message) {
      var item = document.createElement("li");
      var time = document.createElement("time");
      time.dateTime = message.at;
      time.textContent = new Date(message.at).toLocaleString("id-ID", { day: "2-digit", month: "short", hour: "2-digit", minute: "2-digit" });
      var text = document.createElement("span");
      text.textContent = message.text;
      item.append(time, text);
      list.appendChild(item);
    });
  }
  function load() {
    fetch("/history.json").then(function (response) {
      return response.ok ? response.json() : { messages: [] };
    }).then(function (payload) {
      render(payload && payload.messages ? payload.messages : []);
    }).catch(function () {
      list.textContent = "";
      var empty = document.createElement("li");
      empty.className = "empty";
      empty.textContent = "Riwayat belum tersedia.";
      list.appendChild(empty);
    });
  }
  load();
  setInterval(load, 30000);
})();
</script>`

// brandBlock is the same header on both pages: a round profile-style logo with
// the product name beside it.
const brandBlock = `<header class="brand">
<img src="/logo.jpg" alt="Logo JoyX" width="40" height="40">
<span class="name">JoyX</span>
<span class="version">v1</span>
</header>
`

// creditBlock is the foot of both pages: one box for the maker's Instagram and
// one for the GitHub repository.
const creditBlock = `<footer class="credits">
<a class="credit" href="` + razzLink + `" target="_blank" rel="noopener noreferrer" aria-label="RazzX di Instagram">
<svg viewBox="0 0 448 512" width="18" height="18" aria-hidden="true" focusable="false"><path d="M224.1 141c-63.6 0-114.9 51.3-114.9 114.9s51.3 114.9 114.9 114.9S339 319.5 339 255.9 287.7 141 224.1 141zm0 189.6c-41.1 0-74.7-33.5-74.7-74.7s33.5-74.7 74.7-74.7 74.7 33.5 74.7 74.7-33.6 74.7-74.7 74.7zm146.4-194.3c0 14.9-12 26.8-26.8 26.8-14.9 0-26.8-12-26.8-26.8s12-26.8 26.8-26.8 26.8 12 26.8 26.8zm76.1 27.2c-1.7-35.9-9.9-67.7-36.2-93.9-26.2-26.2-58-34.4-93.9-36.2-37-2.1-147.9-2.1-184.9 0-35.8 1.7-67.6 9.9-93.9 36.1s-34.4 58-36.2 93.9c-2.1 37-2.1 147.9 0 184.9 1.7 35.9 9.9 67.7 36.2 93.9s58 34.4 93.9 36.2c37 2.1 147.9 2.1 184.9 0 35.9-1.7 67.7-9.9 93.9-36.2 26.2-26.2 34.4-58 36.2-93.9 2.1-37 2.1-147.8 0-184.8zM398.8 388c-7.8 19.6-22.9 34.7-42.6 42.6-29.5 11.7-99.5 9-132.1 9s-102.7 2.6-132.1-9c-19.6-7.8-34.7-22.9-42.6-42.6-11.7-29.5-9-99.5-9-132.1s-2.6-102.7 9-132.1c7.8-19.6 22.9-34.7 42.6-42.6 29.5-11.7 99.5-9 132.1-9s102.7-2.6 132.1 9c19.6 7.8 34.7 22.9 42.6 42.6 11.7 29.5 9 99.5 9 132.1s2.7 102.7-9 132.1z"/></svg>
<span>RazzX</span></a>
<a class="credit" href="` + githubLink + `" target="_blank" rel="noopener noreferrer" aria-label="EllMaLuvv di GitHub">
<svg viewBox="0 0 16 16" width="18" height="18" aria-hidden="true" focusable="false"><path d="M8 0C3.58 0 0 3.58 0 8c0 3.54 2.29 6.53 5.47 7.59.4.07.55-.17.55-.38 0-.19-.01-.82-.01-1.49-2.01.37-2.53-.49-2.69-.94-.09-.23-.48-.94-.82-1.13-.28-.15-.68-.52-.01-.53.63-.01 1.08.58 1.23.82.72 1.21 1.87.87 2.33.66.07-.52.28-.87.51-1.07-1.78-.2-3.64-.89-3.64-3.95 0-.87.31-1.59.82-2.15-.08-.2-.36-1.02.08-2.12 0 0 .67-.21 2.2.82.64-.18 1.32-.27 2-.27s1.36.09 2 .27c1.53-1.04 2.2-.82 2.2-.82.44 1.1.16 1.92.08 2.12.51.56.82 1.27.82 2.15 0 3.07-1.87 3.75-3.65 3.95.29.25.54.73.54 1.48 0 1.07-.01 1.93-.01 2.2 0 .21.15.46.55.38A8.01 8.01 0 0 0 16 8c0-4.42-3.58-8-8-8Z"/></svg>
<span>EllMaLuvv</span></a>
</footer>
`

const pairingPage = `<!doctype html>
<html lang="id">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>JoyX v1 — sambungkan WhatsApp</title>
<style>` + pairingStyles + `</style>
<noscript><meta http-equiv="refresh" content="5"></noscript>
<script>
// Auto-refresh keeps the QR current, but never under reduced motion: the page
// then waits for the manual reload link instead.
if (!window.matchMedia("(prefers-reduced-motion: reduce)").matches) {
  setTimeout(function () { window.location.reload(); }, 5000);
}
</script>
</head>
<body>
` + historyBlock + `
` + brandBlock + `<main class="card" id="card">
<h1>Sambungkan WhatsApp JoyX</h1>
<img class="qr" src="/qr.png" alt="Kode QR untuk menautkan WhatsApp JoyX" width="336" height="336">
<p class="status" aria-live="polite">Kode berlaku {{countdown}}. Halaman ini diperbarui otomatis.</p>
<div class="actions">
  <a class="button" href="/qr.png">Buka gambar QR</a>
  <a class="plain" href="/">Muat ulang</a>
</div>
<p>Scan dari HP: WhatsApp &gt; <strong>Perangkat Tertaut</strong>.</p>
</main>
` + creditBlock + `
<script>
// One quiet bit of life: a soft light that follows the pointer across the
// card. Paint-only (custom properties), so nothing reflows, and it stays off
// under reduced motion.
if (!window.matchMedia("(prefers-reduced-motion: reduce)").matches) {
  document.getElementById("card").addEventListener("pointermove", function (event) {
    var box = event.currentTarget.getBoundingClientRect();
    event.currentTarget.style.setProperty("--gx", ((event.clientX - box.left) / box.width * 100) + "%");
    event.currentTarget.style.setProperty("--gy", ((event.clientY - box.top) / box.height * 100) + "%");
  });
}
</script>
</body>
</html>
`

const pairingPageWaiting = `<!doctype html>
<html lang="id">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>JoyX v1 — bot berjalan</title>
<style>` + pairingStyles + `</style>
<noscript><meta http-equiv="refresh" content="2"></noscript>
<script>
if (!window.matchMedia("(prefers-reduced-motion: reduce)").matches) {
  setTimeout(function () { window.location.reload(); }, 2000);
}
</script>
</head>
<body>
` + historyBlock + `
` + brandBlock + `<main class="card">
<h1>JoyX v1</h1>
<p class="status" aria-live="polite">Bot berjalan. Tidak ada sesi pairing yang aktif.</p>
<p>Kalau JoyX belum tersambung, QR akan muncul di sini dan diperbarui otomatis.</p>
</main>
` + creditBlock + `
</body>
</html>
`

// MultiPairingSink fans one pairing event out to several sinks, so a caller
// can show the QR in a terminal and in a browser at once.
type MultiPairingSink []PairingSink

// ShowPairingCode calls every sink. A failure in one does not stop the others.
func (sinks MultiPairingSink) ShowPairingCode(code string, validFor time.Duration) error {
	var joined error
	for _, sink := range sinks {
		if sink == nil {
			continue
		}
		joined = errors.Join(joined, sink.ShowPairingCode(code, validFor))
	}
	return joined
}
