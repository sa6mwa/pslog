# pslogconsole2json

Convert pslog console logs (plain or color) into pslog-style NDJSON with
optional LQL filtering.

## Installation

```bash
go install github.com/sa6mwa/pslog/pslogconsole2json@latest
```

## Usage

```bash
pslogconsole2json [flags] <file> [file...]
pslogconsole2json -i [flags]
```

## Flags

- `-i`  
  Read input from stdin. Must not be combined with input files.
- `-o`  
  Write output files instead of stdout. Each input file is written to
  `<input>.json` unless `-outdir` is set.
- `-outdir <dir>`  
  Directory for outputs when `-o` is set. Created if missing.
- `-l <expr>`  
  LQL selector filter (repeatable). All selectors are ANDed by default.
- `-or`  
  Combine `-l` selectors using OR instead of AND.

## Examples

Convert a file to stdout:

```bash
pslogconsole2json app.log
```

Convert multiple files and write outputs next to inputs:

```bash
pslogconsole2json -o app.log worker.log
```

Write outputs into a directory:

```bash
pslogconsole2json -o -outdir out app.log worker.log
```

Read from stdin:

```bash
cat app.log | pslogconsole2json -i
```

Filter output using LQL:

```bash
pslogconsole2json -l '/lvl=debug' -l '/status>=400' app.log
```

Use OR across selectors:

```bash
pslogconsole2json -or -l '/lvl=error' -l '/status>=500' app.log
```

## Notes

- Output fields are ordered to match pslog JSON: `ts`, `lvl`, `msg`, then the
  remaining fields in console order.
- Timestamps are normalized to RFC3339 where possible. Common formats and
  epoch timestamps are supported.
- Values are heuristically typed (bool, int, float, null, string). Per-key
  typing follows the most recently observed type.
