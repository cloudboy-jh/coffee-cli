# Coffee CLI Specification

## Overview

`coffee` is a tiny macOS CLI that makes it easy to temporarily keep a Mac awake.

The primary goal is a memorable interface for a common action:

```text
coffee
```

The command should toggle the current state and print a clear confirmation.

## Goals

- Provide a simple `coffee` command for macOS.
- Use Go and produce a small, self-contained binary.
- Keep the default behavior temporary and safe.
- Use macOS's built-in `caffeinate` command rather than reimplementing power management.
- Make the current state easy to understand.
- Support installation through a future Homebrew formula.

## Non-goals for v1

- Menu bar UI.
- Cross-platform support.
- Configuration files.
- Timers or scheduled activation.
- Managing arbitrary child commands.
- Changing persistent `pmset` settings by default.

## Command Interface

### Toggle

```bash
coffee
```

If Coffee is off, turn it on. If Coffee is on, turn it off.

Expected output:

```text
☕ Brewing
```

or:

```text
☕ Resting
```

### Explicit state changes

```bash
coffee on
coffee off
```

These commands must be idempotent. Running `coffee on` when it is already on should leave it on and return success. The same applies to `coffee off`.

Expected output after `coffee on`, including when already active:

```text
☕ Brewing
```

Expected output after `coffee off`, including when already inactive:

```text
☕ Resting
```

### Status

```bash
coffee status
```

Expected output:

```text
☕ Brewing
```

or:

```text
☕ Resting
```

`☕ Brewing` means Coffee is active and a Coffee-owned `caffeinate` process is running.

`☕ Resting` means Coffee is inactive and no Coffee-owned `caffeinate` process is running.

### Help and version

```bash
coffee --help
coffee --version
```

`-h` may be accepted as an alias for `--help`.

## Behavior

When turned on, Coffee starts a background macOS `caffeinate` process with an assertion that prevents idle system sleep. The process remains active until `coffee off` is run or the process exits.

The implementation must:

- Avoid requiring `sudo` for the default mode.
- Track the child process reliably.
- Detect and clean up stale process state.
- Handle `SIGINT` and `SIGTERM` by stopping its child process.
- Avoid starting duplicate `caffeinate` processes.
- Return a non-zero exit code for invalid commands or operational failures.
- Print errors to stderr.

The initial implementation should not use `sudo pmset -a disablesleep 1`. Lid-closed sleep prevention may be considered as a separate, explicitly opt-in feature later.

## State

The first version stores runtime state at:

```text
~/Library/Application Support/coffee/coffee.pid
```

The state file includes the Coffee-owned child PID, process command, and process start time so Coffee can guard against PID reuse before stopping a process.

The state directory should be created on demand. If the state file points to a process that no longer exists or no longer matches the recorded identity, Coffee should treat the state as off and remove the stale file.

- `coffee status` reports the real running state.
- `coffee off` does not kill unrelated processes.
- A stale state file does not prevent recovery.

## macOS Requirements

- macOS with the built-in `caffeinate` command.
- No third-party runtime dependencies.
- Initial release targets Apple Silicon and Intel macOS through Go builds.

## Installation

The primary v1 installation path is Homebrew:

```bash
brew install --HEAD cloudboy-jh/tap/coffee
```

The formula name is `coffee`. The tap target is `cloudboy-jh/tap`.

Stable Homebrew installation should be added after the first tagged release:

```bash
brew install cloudboy-jh/tap/coffee
```

## Exit Codes

- `0` — successful operation.
- `1` — operational failure, such as inability to start or stop `caffeinate`.
- `2` — invalid command or argument.

## Example Session

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

## Future Considerations

Possible follow-up features, only if the basic CLI proves useful:

- `coffee for 1h` for timed activation.
- `coffee run -- <command>` to keep the Mac awake while a command runs.
- An explicit `--lid` mode backed by `pmset`, with clear privilege and safety warnings.

## Success Criteria

The v1 release is successful when a user can install one binary, run `coffee`, and reliably toggle temporary sleep prevention without remembering `caffeinate` flags or using `sudo`.

The implementation should remain small, understandable, and boring.


## License

To be decided before the first release.

## Status

Implemented as a Go CLI with a Homebrew formula.

Last updated: 2026-08-17
