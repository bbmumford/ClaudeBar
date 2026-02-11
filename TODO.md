# ClaudeBar - TODO

## Cross-Platform: Linux

### Platform Features (`internal/platform/linux.go`)
- [ ] **Global hotkeys** - Currently stubbed. Implement via X11 `XGrabKey` API (requires CGO with `libX11`) or `xbindkeys` integration. Wayland has no global hotkey standard yet
- [ ] **Always-on-top** - Uses `wmctrl`/`xdotool` CLI tools. Works but requires user to install them (`sudo apt install wmctrl xdotool`). Native X11 via `_NET_WM_STATE_ABOVE` would be more reliable
- [ ] **Transparency** - Uses `xprop` to set `_NET_WM_WINDOW_OPACITY`. Requires compositor (most modern DEs have one). Won't work on bare X11 without compositor
- [ ] **Window move/resize** - Uses `xdotool`. Needs to be installed. Native X11 `XMoveResizeWindow` via CGO would be better
- [ ] **Idle detection** - Uses `xprintidle` CLI tool. Needs install (`sudo apt install xprintidle`). Native X11 `XScreenSaverQueryInfo` via CGO is preferred
- [ ] **Wayland support** - All X11 tools (xdotool, wmctrl, xprop, xprintidle) don't work on Wayland. Would need portal/D-Bus APIs or compositor-specific protocols

### Browser Cookies (`internal/browser/cookies_linux.go`)
- [ ] **Chrome key decryption** - Chrome on Linux stores the encryption key in GNOME Keyring (libsecret) or KWallet. Implement via `secret-tool` CLI or CGO with `libsecret-1`
- [ ] **Snap/Flatpak Chrome paths** - Snap and Flatpak Chrome installations use different paths (`~/snap/chromium/...`, `~/.var/app/...`)

### Build
- [ ] **CGO cross-compile** - `go-sqlite3` requires CGO. Need Linux GCC for native builds. Options: build on Linux, use Docker, or use `zig cc` as cross-compiler
- [ ] **Package as .deb/.rpm** - Create proper Linux packages with desktop file and icon
- [ ] **AppImage** - Single-file portable Linux binary
- [ ] **System tray** - Fyne systray uses `fyne.io/systray` which works on Linux but may need `libayatana-appindicator3-dev` installed

## Cross-Platform: macOS

### Platform Features (`internal/platform/darwin.go`)
- [ ] **Global hotkeys** - Requires Carbon `RegisterEventHotKey` API via CGO. No pure Go solution
- [ ] **Always-on-top** - Needs `NSWindow.level = .floating` via CGO Objective-C bridge. AppleScript fallback is unreliable
- [ ] **Transparency** - Needs `NSWindow.alphaValue` via CGO. No CLI alternative
- [ ] **Window move/resize** - AppleScript works but is slow. CGO with `NSWindow setFrame:` is preferred
- [ ] **Idle detection** - `ioreg` parsing works but is fragile. CGO with `CGEventSourceSecondsSinceLastEventType` is better
- [ ] **GetWindowHandle** - Needs `CGWindowListCopyWindowInfo` via CGO. Without it, always-on-top and transparency won't work

### Browser Cookies (`internal/browser/cookies_darwin.go`)
- [ ] **Chrome key decryption** - Chrome on macOS stores the key in Keychain under "Chrome Safe Storage". Implement via `security find-generic-password` CLI or CGO with Security framework
- [ ] **Safari cookies** - macOS users may use Safari. Cookie DB at `~/Library/Cookies/Cookies.binarycookies` (different format)

### Build
- [ ] **CGO cross-compile** - Need macOS SDK and `osxcross` toolchain for cross-compilation from Windows/Linux
- [ ] **Code signing** - macOS Gatekeeper requires signed apps. Unsigned apps need user to right-click > Open
- [ ] **DMG packaging** - Create .dmg with drag-to-Applications installer
- [ ] **.app bundle** - Fyne can create .app bundles via `fyne package`
- [ ] **Menu bar app** - macOS convention is menu bar (not system tray). Fyne supports this

## General

### UI Improvements
- [ ] **Dynamic overlay sizing** - Overlay size/placement could be more responsive to screen DPI and resolution
- [ ] **Fyne native transparency** - Monitor Fyne releases for native window transparency support (would eliminate platform-specific code)

### Authentication
- [ ] **Production exe timeout** - Increased timeout to 60s + retry, but root cause (possibly Windows Firewall blocking GUI-subsystem exe) needs investigation
- [ ] **OAuth flow** - Implement proper OAuth with claude.ai instead of manual session key paste
- [ ] **Auto-refresh session** - Detect expired session and prompt user to re-enter key

### Features
- [ ] **Auto-start on login** - Windows: registry key, Linux: .desktop autostart, macOS: Login Items
- [ ] **Notification on high usage** - Alert when approaching rate limits
- [ ] **Model-specific usage** - Display Opus/Sonnet breakdowns (data already fetched but not shown)
