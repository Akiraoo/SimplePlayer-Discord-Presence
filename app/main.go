// SimplePlayerPresence.exe — background helper for the "Simple Player Discord Presence"
// Chrome extension.
//
// Double-click it once to install: it copies itself to %LOCALAPPDATA%\SimplePlayerPresence,
// starts with Windows (HKCU Run key, no admin rights), and unpacks the extension next to it.
// While running it listens on 127.0.0.1 for the extension and forwards the activity to the
// Discord desktop app through Discord's local IPC pipe, so no Discord login is needed.
package main

import (
	"embed"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

const (
	appName       = "SimplePlayerPresence"
	listenAddr    = "127.0.0.1:47823"
	defaultClient = "1552923544420225046"
	// The extension re-sends its state every 20 s while a song plays. If nothing arrives
	// for this long (browser closed, tab crashed), the Discord status is cleared.
	staleAfter = 75 * time.Second
	retryEvery = 15 * time.Second
)

//go:embed all:extension
var extensionFiles embed.FS

func installDir() string {
	base := os.Getenv("LOCALAPPDATA")
	if base == "" {
		base, _ = os.UserConfigDir()
	}
	return filepath.Join(base, appName)
}

func installedExe() string { return filepath.Join(installDir(), appName+".exe") }

func main() {
	arg := ""
	if len(os.Args) > 1 {
		arg = strings.ToLower(os.Args[1])
	}
	switch arg {
	case "--background":
		runServer()
	case "--uninstall":
		uninstall(true)
	case "--install":
		install()
	default:
		self, _ := os.Executable()
		if samePath(self, installedExe()) {
			if confirm("Simple Player Discord Presence 已安裝並在背景執行。\n\n要解除安裝嗎？") {
				uninstall(false)
			} else {
				startBackground()
			}
			return
		}
		install()
	}
}

func samePath(a, b string) bool {
	ea, _ := filepath.EvalSymlinks(a)
	eb, _ := filepath.EvalSymlinks(b)
	if ea == "" {
		ea = a
	}
	if eb == "" {
		eb = b
	}
	return strings.EqualFold(filepath.Clean(ea), filepath.Clean(eb))
}

/* ---------------- install / uninstall ---------------- */

func install() {
	stopRunning()
	dir := installDir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		alert("安裝失敗：" + err.Error())
		return
	}
	self, _ := os.Executable()
	if !samePath(self, installedExe()) {
		if err := copyFile(self, installedExe()); err != nil {
			alert("安裝失敗（複製程式）：" + err.Error())
			return
		}
	}
	extDir := filepath.Join(dir, "extension")
	_ = os.RemoveAll(extDir)
	if err := writeExtension(extDir); err != nil {
		alert("安裝失敗（擴充功能）：" + err.Error())
		return
	}
	if err := setAutostart(true); err != nil {
		alert("安裝失敗（開機自動啟動）：" + err.Error())
		return
	}
	startBackground()

	// Chrome only lets users add an extension that is not from the Web Store by hand,
	// so open the extension page and the folder to make that one step easy.
	openFolder(extDir)
	openChromeExtensions()
	alert("安裝完成，之後會隨 Windows 自動在背景執行。\n\n" +
		"最後一步（只要做一次）：\n" +
		"1. 在剛打開的 Chrome「擴充功能」頁面，開啟右上角的「開發人員模式」\n" +
		"2. 按「載入未封裝項目」，選擇剛打開的資料夾：\n   " + extDir + "\n\n" +
		"之後在電腦版 Simple Player 網頁播放歌曲，Discord 就會顯示狀態。")
}

func uninstall(silent bool) {
	stopRunning()
	_ = setAutostart(false)
	dir := installDir()
	// The running exe may be the one inside dir, so delete the folder after we exit.
	cmd := exec.Command("cmd", "/c", "ping 127.0.0.1 -n 3 >nul & rmdir /s /q \""+dir+"\"")
	hideWindow(cmd)
	_ = cmd.Start()
	if !silent {
		alert("已解除安裝。\n\n記得也到 Chrome 的擴充功能頁面移除「Simple Player Discord Presence」。")
	}
}

func writeExtension(dst string) error {
	return fs.WalkDir(extensionFiles, "extension", func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel := strings.TrimPrefix(p, "extension")
		target := filepath.Join(dst, filepath.FromSlash(rel))
		if d.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		b, err := extensionFiles.ReadFile(p)
		if err != nil {
			return err
		}
		return os.WriteFile(target, b, 0o644)
	})
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	tmp := dst + ".new"
	out, err := os.Create(tmp)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		return err
	}
	if err := out.Close(); err != nil {
		return err
	}
	_ = os.Remove(dst)
	return os.Rename(tmp, dst)
}

func setAutostart(on bool) error {
	key := `HKCU\Software\Microsoft\Windows\CurrentVersion\Run`
	var cmd *exec.Cmd
	if on {
		cmd = exec.Command("reg", "add", key, "/v", appName, "/t", "REG_SZ",
			"/d", fmt.Sprintf(`"%s" --background`, installedExe()), "/f")
	} else {
		cmd = exec.Command("reg", "delete", key, "/v", appName, "/f")
	}
	hideWindow(cmd)
	out, err := cmd.CombinedOutput()
	if err != nil && on {
		return fmt.Errorf("%v %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

func startBackground() {
	if isRunning() {
		return
	}
	cmd := exec.Command(installedExe(), "--background")
	hideWindow(cmd)
	_ = cmd.Start()
}

func isRunning() bool {
	c, err := net.DialTimeout("tcp", listenAddr, 500*time.Millisecond)
	if err != nil {
		return false
	}
	c.Close()
	return true
}

func stopRunning() {
	if !isRunning() {
		return
	}
	req, _ := http.NewRequest("POST", "http://"+listenAddr+"/quit", nil)
	req.Header.Set("X-SimplePlayer", "1")
	client := &http.Client{Timeout: 2 * time.Second}
	if resp, err := client.Do(req); err == nil {
		resp.Body.Close()
	}
	for i := 0; i < 20 && isRunning(); i++ {
		time.Sleep(150 * time.Millisecond)
	}
}

func openFolder(dir string) {
	cmd := exec.Command("explorer", dir)
	_ = cmd.Start()
}

func openChromeExtensions() {
	for _, p := range chromePaths() {
		if _, err := os.Stat(p); err == nil {
			_ = exec.Command(p, "chrome://extensions/").Start()
			return
		}
	}
}

func chromePaths() []string {
	var out []string
	for _, base := range []string{os.Getenv("ProgramFiles"), os.Getenv("ProgramFiles(x86)"), os.Getenv("LOCALAPPDATA")} {
		if base != "" {
			out = append(out, filepath.Join(base, `Google\Chrome\Application\chrome.exe`))
		}
	}
	return out
}

/* ---------------- background server ---------------- */

type state struct {
	mu       sync.Mutex
	clientID string
	activity json.RawMessage // nil = clear
	lastSeen time.Time
	discord  *ipc
}

func runServer() {
	ln, err := net.Listen("tcp", listenAddr)
	if err != nil {
		return // already running
	}
	st := &state{clientID: defaultClient}
	st.discord = newIPC(st.clientID)
	go st.discord.loop()
	go st.watchdog()

	mux := http.NewServeMux()
	mux.HandleFunc("/presence", st.handlePresence)
	mux.HandleFunc("/status", st.handleStatus)
	mux.HandleFunc("/quit", func(w http.ResponseWriter, r *http.Request) {
		if !allowed(r) {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		w.WriteHeader(http.StatusNoContent)
		go func() {
			st.discord.clear()
			time.Sleep(200 * time.Millisecond)
			os.Exit(0)
		}()
	})
	_ = http.Serve(ln, mux)
}

// Only the extension (or this program) may talk to us. Web pages cannot add the custom
// header without a CORS preflight, which we never approve, and their Origin is not an
// extension origin.
func allowed(r *http.Request) bool {
	if r.Method != http.MethodPost && r.Method != http.MethodGet {
		return false
	}
	if r.Header.Get("X-SimplePlayer") != "1" {
		return false
	}
	o := r.Header.Get("Origin")
	return o == "" || strings.HasPrefix(o, "chrome-extension://") || strings.HasPrefix(o, "extension://")
}

func (st *state) handlePresence(w http.ResponseWriter, r *http.Request) {
	if !allowed(r) || r.Method != http.MethodPost {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	var body struct {
		ClientID string          `json:"clientId"`
		Activity json.RawMessage `json:"activity"`
	}
	if err := json.NewDecoder(io.LimitReader(r.Body, 64<<10)).Decode(&body); err != nil {
		http.Error(w, "bad json", http.StatusBadRequest)
		return
	}
	act := body.Activity
	if len(act) == 0 || string(act) == "null" {
		act = nil
	}
	st.mu.Lock()
	st.lastSeen = time.Now()
	st.activity = act
	if body.ClientID != "" && body.ClientID != st.clientID {
		st.clientID = body.ClientID
		st.discord.setClientID(body.ClientID)
	}
	st.mu.Unlock()
	st.discord.setActivity(act)
	st.handleStatus(w, r)
}

func (st *state) handleStatus(w http.ResponseWriter, r *http.Request) {
	if !allowed(r) {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	connected, user, errText := st.discord.status()
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"connected": connected, "user": user, "error": errText, "version": version,
	})
}

func (st *state) watchdog() {
	for range time.Tick(10 * time.Second) {
		st.mu.Lock()
		stale := st.activity != nil && time.Since(st.lastSeen) > staleAfter
		if stale {
			st.activity = nil
		}
		st.mu.Unlock()
		if stale {
			st.discord.setActivity(nil)
		}
	}
}

const version = "1.0.0"
