# dstat

Just a little Go project to see what’s in your folders.

## Clone

```bash
git clone https://github.com/YourUsername/filescanner.git
cd filescanner
```

## Build

```bash
go build -o filescanner main.go
```

## Add to PATH (optional)

```bash
# Unix-like
sudo mv filescanner /usr/local/bin/
# now you can just run `filescanner` from anywhere
```

## Usage

```bash
filescanner [directory] [flags]
```

## Or else

Just run `./make` which contains:

```bash
#!/bin/bash
set -e
go build .
sudo mv dstat /usr/local/bin/
```

If no directory is given, it defaults to `.` (current folder).

### Flags

- `--verbose` : Don’t collapse tiny percentages into "other".
- `--debar` : No fancy bars, just percentages.
- `--size` : Show total directory size.
- `--sizeonly` : Only print the total size, nothing else.
- `--include-hidden`: Include hidden files.
- `--human` : Round percentages to whole numbers.
- `--minsize <n>` : Only include files >= n bytes.
- `--maxsize <n>` : Only include files <= n bytes.
- `--exclude <ext>`: Comma-separated list of extensions to ignore.
- `--bysize` : Calculate percentages based on file sizes instead of counts.

---

### Example

```bash
filescanner ~/projects --size --bysize --human
```

That’s it. Run it, ignore it, modify it, whatever.

