package main

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

// Config holds command-line options
// Controls which directory is scanned and how results are filtered/shown
type Config struct {
	Dir           string
	Verbose       bool
	NoBar         bool
	ShowSize      bool
	SizeOnly      bool
	IncludeHidden bool
	Human         bool
	MinSize       int64
	MaxSize       int64
	Exclude       map[string]struct{}
	ExcludeDirs   map[string]struct{}
	BySize        bool
}

// FileStat stores aggregated file statistics for an extension
// Ext = file extension, Count = number of files, Size = cumulative bytes
type FileStat struct {
	Ext   string
	Count int
	Size  int64
}

// help string for CLI usage
var helpString = `
Usage: file-stats [options] [directory]

Options:
    --verbose           Show all file types, including those <1%.
    --nobar             Suppress bar chart output, print percentages only.
    --size              Print total directory size.
    --sizeonly          Only print directory size and exit.
    --include-hidden    Include hidden files in stats.
    --human             Round percentages to whole numbers.
    --minsize <bytes>   Only include files >= this size.
    --maxsize <bytes>   Only include files <= this size.
    --exclude <exts>    Comma-separated list of extensions to exclude.
    --excludedir <dirs> Comma-separated list of directory names to exclude.
    --bysize            Sort results by file size instead of count.
    --help              Show this help.
`

func main() {
	cfg, err := parseArgs(os.Args)
	if err != nil {
		fmt.Fprintln(os.Stderr, "Error:", err)
		os.Exit(1)
	}

	if cfg == nil {
		// --help requested, exit cleanly
		os.Exit(0)
	}

	counts, sizeCounts, total, totalBytes, err := walkDir(*cfg)
	if err != nil {
		fmt.Fprintln(os.Stderr, "Error walking directory:", err)
		os.Exit(1)
	}

	if cfg.SizeOnly {
		fmt.Println(humanReadableSize(totalBytes))
		return
	}

	if cfg.ShowSize {
		fmt.Printf("Directory size: %s\n", humanReadableSize(totalBytes))
	}

	if total == 0 && totalBytes == 0 {
		fmt.Println("No files matched criteria.")
		return
	}

	stats := aggregateStats(*cfg, counts, sizeCounts, total, totalBytes)
	printStats(*cfg, stats, total, totalBytes)
}

// parseArgs converts command-line args into a Config struct
// Supports both "--flag value" and "--flag=value" forms
func parseArgs(args []string) (*Config, error) {
	cfg := &Config{
		Dir:         ".",
		Exclude:     make(map[string]struct{}),
		ExcludeDirs: make(map[string]struct{}),
	}

	for i := 1; i < len(args); i++ {
		arg := args[i]

		// Support --key=value form
		if strings.Contains(arg, "=") {
			parts := strings.SplitN(arg, "=", 2)
			key := parts[0]
			val := parts[1]
			switch key {
			case "--exclude":
				for _, ext := range strings.Split(val, ",") {
					cfg.Exclude[strings.TrimPrefix(strings.TrimSpace(ext), ".")] = struct{}{}
				}
			case "--excludedir":
				for _, dir := range strings.Split(val, ",") {
					cfg.ExcludeDirs[strings.TrimSpace(dir)] = struct{}{}
				}
			case "--minsize":
				n, err := strconv.ParseInt(val, 10, 64)
				if err != nil {
					return nil, fmt.Errorf("invalid --minsize value: %v", err)
				}
				cfg.MinSize = n
			case "--maxsize":
				n, err := strconv.ParseInt(val, 10, 64)
				if err != nil {
					return nil, fmt.Errorf("invalid --maxsize value: %v", err)
				}
				cfg.MaxSize = n
			default:
				// ignore unknown --key=value
			}
			continue
		}

		switch arg {
		case "--verbose":
			cfg.Verbose = true
		case "--nobar":
			cfg.NoBar = true
		case "--size":
			cfg.ShowSize = true
		case "--sizeonly":
			cfg.SizeOnly = true
		case "--include-hidden":
			cfg.IncludeHidden = true
		case "--human":
			cfg.Human = true
		case "--minsize":
			if i+1 >= len(args) {
				return nil, fmt.Errorf("--minsize requires a value")
			}
			i++
			n, err := strconv.ParseInt(args[i], 10, 64)
			if err != nil {
				return nil, fmt.Errorf("invalid --minsize value: %v", err)
			}
			cfg.MinSize = n
		case "--maxsize":
			if i+1 >= len(args) {
				return nil, fmt.Errorf("--maxsize requires a value")
			}
			i++
			n, err := strconv.ParseInt(args[i], 10, 64)
			if err != nil {
				return nil, fmt.Errorf("invalid --maxsize value: %v", err)
			}
			cfg.MaxSize = n
		case "--exclude":
			if i+1 >= len(args) {
				return nil, fmt.Errorf("--exclude requires a value")
			}
			i++
			for _, ext := range strings.Split(args[i], ",") {
				cfg.Exclude[strings.TrimPrefix(strings.TrimSpace(ext), ".")] = struct{}{}
			}
		case "--excludedir":
			if i+1 >= len(args) {
				return nil, fmt.Errorf("--excludedir requires a value")
			}
			i++
			for _, dir := range strings.Split(args[i], ",") {
				cfg.ExcludeDirs[strings.TrimSpace(dir)] = struct{}{}
			}
		case "--bysize":
			cfg.BySize = true
		case "--help":
			fmt.Println(helpString)
			return nil, nil
		default:
			cfg.Dir = arg
		}
	}

	return cfg, nil
}

// walkDir scans the directory recursively and counts files by extension
// Applies filters for hidden files, min/max size, and excluded extensions/dirs
func walkDir(cfg Config) (map[string]int, map[string]int64, int, int64, error) {
	counts := make(map[string]int)
	sizeCounts := make(map[string]int64)
	var total int
	var totalBytes int64

	err := filepath.WalkDir(cfg.Dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			fmt.Fprintln(os.Stderr, "Skipping", path, "due to error:", err)
			return nil
		}
		if d.IsDir() {
			if _, skip := cfg.ExcludeDirs[d.Name()]; skip {
				return filepath.SkipDir
			}
			return nil
		}
		if !cfg.IncludeHidden && strings.HasPrefix(d.Name(), ".") {
			return nil
		}

		info, err := d.Info()
		if err != nil {
			fmt.Fprintln(os.Stderr, "Skipping", path, "due to error:", err)
			return nil
		}

		if (cfg.MinSize > 0 && info.Size() < cfg.MinSize) || (cfg.MaxSize > 0 && info.Size() > cfg.MaxSize) {
			return nil
		}

		ext := filepath.Ext(d.Name())
		if ext == "" {
			ext = "[noext]"
		} else {
			ext = strings.TrimPrefix(ext, ".")
		}

		if _, skip := cfg.Exclude[ext]; skip {
			return nil
		}

		totalBytes += info.Size()
		counts[ext]++
		sizeCounts[ext] += info.Size()
		total++
		return nil
	})

	return counts, sizeCounts, total, totalBytes, err
}

// aggregateStats groups small categories into "other" unless --verbose is set
// Sorts results by count or by size depending on cfg.BySize
func aggregateStats(cfg Config, counts map[string]int, sizeCounts map[string]int64, total int, totalBytes int64) []FileStat {
	stats := []FileStat{}

	if cfg.BySize {
		var other int64
		for k, v := range sizeCounts {
			percent := safeDivF(float64(v), float64(totalBytes))
			if !cfg.Verbose && percent < 0.01 {
				other += v
			} else {
				stats = append(stats, FileStat{k, 0, v})
			}
		}
		if other > 0 {
			stats = append(stats, FileStat{"other", 0, other})
		}
		sort.Slice(stats, func(i, j int) bool {
			return stats[i].Size > stats[j].Size
		})
	} else {
		var other int
		for k, v := range counts {
			percent := safeDivF(float64(v), float64(total))
			if !cfg.Verbose && percent < 0.01 {
				other += v
			} else {
				stats = append(stats, FileStat{k, v, 0})
			}
		}
		if other > 0 {
			stats = append(stats, FileStat{"other", other, 0})
		}
		sort.Slice(stats, func(i, j int) bool {
			return stats[i].Count > stats[j].Count
		})
	}

	return stats
}

// printStats displays the results with ASCII bar chart unless --nobar is set
func printStats(cfg Config, stats []FileStat, total int, totalBytes int64) {
	barWidth := 40
	if !cfg.NoBar {
		fmt.Println("File type breakdown:")
	}

	for _, s := range stats {
		var percent float64
		if cfg.BySize {
			percent = safeDivF(float64(s.Size), float64(totalBytes)) * 100
		} else {
			percent = safeDivF(float64(s.Count), float64(total)) * 100
		}

		if cfg.Human {
			percent = float64(int(percent + 0.5))
		}

		if cfg.NoBar {
			fmt.Printf("%-10s %5.0f%%\n", s.Ext, percent)
		} else {
			barLen := int(percent / 100 * float64(barWidth))
			bar := strings.Repeat("█", barLen) + strings.Repeat("-", barWidth-barLen)
			fmt.Printf("%-10s |%s| %5.2f%%\n", s.Ext, bar, percent)
		}
	}
}

// humanReadableSize formats a byte count into KB/MB/GB/TB string
func humanReadableSize(bytes int64) string {
	const (
		KB = 1024
		MB = 1024 * KB
		GB = 1024 * MB
		TB = 1024 * GB
	)

	switch {
	case bytes >= TB:
		return fmt.Sprintf("%.2f TB", float64(bytes)/TB)
	case bytes >= GB:
		return fmt.Sprintf("%.2f GB", float64(bytes)/GB)
	case bytes >= MB:
		return fmt.Sprintf("%.2f MB", float64(bytes)/MB)
	case bytes >= KB:
		return fmt.Sprintf("%.2f KB", float64(bytes)/KB)
	default:
		return fmt.Sprintf("%d B", bytes)
	}
}

// safeDivF does floating point division with zero check
func safeDivF(a, b float64) float64 {
	if b == 0 {
		return 0
	}
	return a / b
}

