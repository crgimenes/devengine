# filo-cli

Command-line tool for running Filo scripts.

## Building

```bash
go build -o filo-cli ./cmd/filo-cli
```

## Usage

```bash
filo-cli [options]
```

### Basic Usage (No Database Required)

Simple scripts run without any database configuration:

```bash
# Simple expression via stdin
echo '(+ 1 2 3)' | ./filo-cli
# Output: 6

# With math package
echo '(sqrt 16)' | ./filo-cli --filo-package math
# Output: 4

# From file
./filo-cli --script-file calculate.filo
```

### With Database Context

For EAV access, provide database credentials:

```bash
./filo-cli --db app.db --email admin@example.com --workspace-id 1 --filo-package eav
```

## Flags

### Script Input

| Flag | Default | Description |
|------|---------|-------------|
| `--script-file` | (stdin) | Path to Filo script file |

### Database Context (Required for eav package)

| Flag | Description |
|------|-------------|
| `--db` | Path to SQLite database file |
| `--email` | Email of the user to load |
| `--workspace-id` | Workspace ID to use |
| `--workspace-ref` | Workspace reference ID (alternative) |

### Extension Packages

| Flag | Default | Description |
|------|---------|-------------|
| `--filo-package` | (none) | Comma-separated list: `eav`, `math` |

### Execution Limits

| Flag | Default | Description |
|------|---------|-------------|
| `--step-limit` | 100000 | Maximum evaluation steps |
| `--recursion-limit` | 128 | Maximum recursion depth |
| `--timeout` | 30 | Timeout in seconds |

## Extension Packages

| Package | Requires DB | Description |
|---------|-------------|-------------|
| `math` | No | Advanced math: sqrt, sin, cos, log, etc. |
| `eav` | Yes | EAV data access builtins |

## Examples

### Pure Filo (No Dependencies)

```bash
# Arithmetic
echo '(* 2 (+ 3 4))' | ./filo-cli
# Output: 14

# Conditionals
echo '(if (> 10 5) "yes" "no")' | ./filo-cli
# Output: yes

# Lists
echo '(head (list 1 2 3))' | ./filo-cli
# Output: 1
```

### With Math Package

```bash
# Trigonometry
echo '(sin (/ (pi) 2))' | ./filo-cli --filo-package math
# Output: 1

# Pythagorean theorem
echo '(sqrt (+ (pow 3 2) (pow 4 2)))' | ./filo-cli --filo-package math
# Output: 5

# Rounding
echo '(round 3.7)' | ./filo-cli --filo-package math
# Output: 4
```

### With EAV Package

```bash
# Get field value
echo '(eav-get-value "products" 123 "price")' | \
  ./filo-cli --db app.db --email admin@example.com --workspace-id 1 --filo-package eav

# Sum with filter
./filo-cli --db app.db --email admin@example.com --workspace-id 1 \
  --filo-package eav,math \
  --script-file analytics.filo
```

## Global Variables

When using database context, these globals are available:

| Variable | Type | Description |
|----------|------|-------------|
| `user-id` | number | Current user's ID |
| `user-email` | string | Current user's email |
| `user-name` | string | Current user's username |
| `workspace-id` | number | Current workspace ID |
| `workspace-ref` | string | Current workspace reference ID |

## Exit Codes

| Code | Meaning |
|------|---------|
| 0 | Success |
| 1 | Error |

## Error Handling

Errors are printed to stderr:

```bash
echo '(undefined-function)' | ./filo-cli
# stderr: error: script execution failed: unknown symbol: undefined-function
# exit code: 1
```
