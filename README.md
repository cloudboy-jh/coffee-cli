# ☕ coffee-cli

<p align="center">
  <img src="assets/hero.png" alt="coffee-cli — keep your Mac awake" width="1200">
</p>

**Keep your Mac awake.**

`coffee` is a tiny macOS CLI that wraps Apple's built-in `caffeinate` behind a friendly, memorable command. Type `coffee`, and your Mac stays awake. Type it again, and it goes back to normal.

No `sudo`, no remembering flags, no third-party dependencies.

---

## Install

```bash
brew install --HEAD cloudboy-jh/tap/coffee
```

Build from source:

```bash
go install github.com/cloudboy-jh/coffee-cli@latest
```

## Usage

```text
$ coffee status
☕ Resting

$ coffee
☕ Brewing

$ coffee status
☕ Brewing

$ coffee
☕ Resting
```

| Command            | What it does                          |
| ------------------ | ------------------------------------- |
| `coffee`           | Toggle between awake and sleeping     |
| `coffee on`        | Keep the Mac awake                    |
| `coffee off`       | Let the Mac sleep again               |
| `coffee status`    | Check whether coffee is brewing       |
| `coffee --version` | Print the version                     |
| `coffee --help`    | Show usage                            |

`coffee on` and `coffee off` are idempotent — running them twice is safe.

## How it works

`coffee` starts a background `caffeinate -i` process (an idle-sleep assertion) and tracks it so it can be stopped cleanly later. State lives at:

```
~/Library/Application Support/coffee/coffee.pid
```

It records the child PID, command, and start time — so it never kills an unrelated process, and a stale state file can't leave you stuck. `SIGINT` / `SIGTERM` stop the child process too.

## Why?

Because `caffeinate -i` is one of those flags I can never remember. `coffee` is the version I won't forget.

## License

To be decided.
