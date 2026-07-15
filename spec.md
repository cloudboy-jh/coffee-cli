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
Coffee on
```

or:

```text
Coffee off
```

### Explicit state changes

```bash
coffee on
coffee off
```

These commands must be idempotent. Running `coffee on` when it is already on should leave it on and return success. The same applies to `coffee off`.

### Status

```bash
coffee status
```

Expected output:

```text
Coffee on
```

or:

```text
Coffee off
```

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

The first version may store the child PID in a small runtime state file, for example:

```text
~/Library/Application Support/coffee/coffee.pid
```

The state directory should be created on demand. If the state file points to a process that no longer exists, Coffee should treat the state as off and remove the stale file.

The exact state mechanism can change during implementation as long as these properties hold:

- `coffee status` reports the real running state.
- `coffee off` does not kill unrelated processes.
- A stale state file does not prevent recovery.

## macOS Requirements

- macOS with the built-in `caffeinate` command.
- No third-party runtime dependencies.
- Initial release targets Apple Silicon and Intel macOS through Go builds.

## Exit Codes

- `0` — successful operation.
- `1` — operational failure, such as inability to start or stop `caffeinate`.
- `2` — invalid command or argument.

## Example Session

```text
$ coffee status
Coffee off

$ coffee
Coffee on

$ coffee status
Coffee on

$ coffee
Coffee off
```

## Future Considerations

Possible follow-up features, only if the basic CLI proves useful:

- `coffee for 1h` for timed activation.
- `coffee run -- <command>` to keep the Mac awake while a command runs.
- An explicit `--lid` mode backed by `pmset`, with clear privilege and safety warnings.
- Homebrew installation and automated release binaries.

## Success Criteria

The v1 release is successful when a user can install one binary, run `coffee`, and reliably toggle temporary sleep prevention without remembering `caffeinate` flags or using `sudo`.

The implementation should remain small, understandable, and boring.

## Open Questions

- Should `coffee` default to preventing only idle system sleep, or also keep the display awake?
- Should the state file use a PID only, or include a process start-time check to guard against PID reuse?
- Should output include a coffee emoji, or remain plain text for scripting and terminal compatibility?
- What Homebrew tap or formula name should be used?
  
These questions should be resolved before implementation, not by expanding the v1 scope unnecessarily.

## License

To be decided before the first release.

## Status

Specification only. No implementation yet.

Last updated: 2026-07-15
