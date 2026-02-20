---
title: Windows Support
sidebar:
  order: 4
description: Known limitations and workarounds when running Boilerplate on Windows.
---

Boilerplate runs natively on Windows. Pre-built binaries are available for `windows_amd64` and can be downloaded from the [GitHub releases page](https://github.com/gruntwork-io/boilerplate/releases).

Most features work identically across macOS, Linux, and Windows. The sections below cover the differences you should be aware of.

## Path Length Limits

Windows imposes a default maximum file path length of **260 characters**. Boilerplate works around this by converting temporary paths to UNC extended-length form (`\\?\`), but deeply nested template dependencies or long project names can still hit this limit in the generated output.

**Recommendations:**

- Keep your `--output-folder` path short (e.g. `C:\out` instead of `C:\Users\myname\Documents\Projects\generated-output`)
- Avoid deeply nesting dependencies when the output folder paths are already long
- On Windows 10 (version 1607+) and Windows 11, you can remove the 260-character limit entirely by enabling the **LongPathsEnabled** registry key or group policy

## Reserved File Names

Windows reserves certain file names that cannot be used for files or folders regardless of extension: `CON`, `PRN`, `AUX`, `NUL`, `COM0`-`COM9`, and `LPT0`-`LPT9`.

If your templates generate files with these names (e.g. `NUL.txt`, `CON.md`), the output will fail on Windows even though it works on macOS and Linux. Avoid using these names in your template file paths and output folder names.

## Pipe Character in File Names

The `|` character is illegal in Windows file names. Boilerplate supports URL-encoded characters in template file paths as a workaround. If you have a template file path that would normally contain `|`, use `%7C` instead.

## Symlinks

Boilerplate uses file copying instead of symlinks when fetching templates. This avoids permission issues on Windows, where creating symlinks requires elevated privileges or developer mode to be enabled. No action is needed on your part — this is handled automatically.

## Shell Helper

The `shell` template helper executes commands using the system shell. On Windows, commands run through `cmd.exe` rather than a Unix shell. Keep this in mind when writing hooks or using `shell` in templates:

- Use Windows-compatible commands or write cross-platform scripts
- Paths in shell commands should use backslashes (`\`) or forward slashes (`/`), which Windows generally accepts
- Environment variable syntax differs: `%VAR%` in `cmd.exe` vs `$VAR` in bash

## SSH-Based Templates

Some integration tests for SSH-based remote templates are skipped on Windows due to file name compatibility issues in certain Git repositories. If you encounter issues cloning remote templates over SSH on Windows, try using HTTPS URLs instead.
