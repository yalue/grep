// This command-line tool supports a small subset of the GNU grep program's
// functionality, using Go's regular expressions. Usage:
//
//    grep [-irvahHo] <expression> [file paths]
package main

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// Holds settings for how the program will run. If paths is nil or empty,
// then the program should run on stdin.
type options struct {
	insensitive   bool
	recursive     bool
	inverse       bool
	onlyMatched   bool
	expression    string
	binaryAsText  bool
	hideFilenames bool
	showFilenames bool
	paths         []string
}

func parseArgs() (*options, error) {
	var toReturn options
	var e error
	flagsRegex := regexp.MustCompile(`^-([irvahHo]+)$`)
	pathIndex := -1
	for i, arg := range os.Args[1:] {
		if arg == "--help" {
			help()
		}
		if !strings.HasPrefix(arg, "-") {
			// The regex will be compiled later, depends on the insensitive
			// flag.
			toReturn.expression = arg
			pathIndex = i + 2
			break
		}
		matchedFlags := flagsRegex.FindStringSubmatch(arg)
		if len(matchedFlags) <= 0 {
			return nil, fmt.Errorf("Invalid argument: %s", arg)
		}
		for _, c := range matchedFlags[1] {
			switch c {
			case 'i':
				toReturn.insensitive = true
			case 'r':
				toReturn.recursive = true
			case 'v':
				toReturn.inverse = true
			case 'o':
				toReturn.onlyMatched = true
			case 'h':
				toReturn.hideFilenames = true
			case 'H':
				toReturn.hideFilenames = false
				toReturn.showFilenames = true
			case 'a':
				toReturn.binaryAsText = true
			}
		}
	}
	if pathIndex < 0 {
		return nil, fmt.Errorf("No regular expression was provided.")
	}
	toReturn.paths = make([]string, 0, 16)
	var pathMatches []string
	for pathIndex < len(os.Args) {
		pathMatches, e = filepath.Glob(os.Args[pathIndex])
		if e != nil {
			return nil, fmt.Errorf("Invalid file path: %s", e)
		}
		toReturn.paths = append(toReturn.paths, pathMatches...)
		pathIndex++
	}
	sort.Strings(toReturn.paths)
	return &toReturn, nil
}

func isDirectory(filename string) (bool, error) {
	f, e := os.Stat(filename)
	if e != nil {
		return false, fmt.Errorf("Error checking if file is dir: %s", e)
	}
	return f.IsDir(), nil
}

// Replaces args.paths with a recursively expanded list of directories, if
// the recursive flag was set. If the flag wasn't set, this does nothing and
// returns nil.
func doDirectoryWalk(args *options) error {
	var e error
	if !args.recursive {
		return nil
	}
	if len(args.paths) == 0 {
		return nil
	}
	// Create a set of paths; these will be sorted later
	newPaths := make(map[string]bool)
	isDir := false
	for _, path := range args.paths {
		isDir, e = isDirectory(path)
		if e != nil {
			return e
		}
		// Hidden files may still be grepped, and will get added to the list
		// here.
		if !isDir {
			newPaths[path] = true
			continue
		}
		filepath.Walk(path, func(child string, info os.FileInfo,
			walkError error) error {
			if walkError != nil {
				return walkError
			}
			// Ignore hidden files, skip hidden directores (these things start
			// with a '.')
			baseName := filepath.Base(child)
			if strings.HasPrefix(baseName, ".") {
				if info.IsDir() {
					return filepath.SkipDir
				}
				return nil
			}
			// Don't append directories to the list of files.
			if info.IsDir() {
				return nil
			}
			newPaths[child] = true
			return nil
		})
	}
	// Convert the set of paths into a slice of paths, sorted alphabetically.
	sortedPaths := make([]string, 0, len(newPaths))
	for k, v := range newPaths {
		if !v {
			continue
		}
		sortedPaths = append(sortedPaths, k)
	}
	sort.Strings(sortedPaths)
	args.paths = sortedPaths
	return nil
}

// Returns s with a trailing newline removed, if it had a newline.
func chomp(s []byte) []byte {
	if len(s) < 1 {
		return s
	}
	if s[len(s)-1] == '\n' {
		s = s[0 : len(s)-1]
	}
	if len(s) < 1 {
		return s
	}
	if s[len(s)-1] == '\r' {
		s = s[0 : len(s)-1]
	}
	return s
}

// Checks a single file for matches against the expression. Returns a slice of
// matched lines, a boolean indicating whether the file is binary, and an error
// if one occurs.
func getFileMatches(args *options, regex *regexp.Regexp,
	file *os.File) ([][]byte, bool, error) {
	var e error
	matchedLines := make([][]byte, 0, 100)
	isBinary := false
	reader := bufio.NewReader(file)
	var line, matched []byte
	for e == nil {
		line, e = reader.ReadBytes('\n')
		if (e != nil) && (e != io.EOF) {
			break
		}
		if (e == io.EOF) && (len(line) == 0) {
			break
		}
		line = chomp(line)
		if bytes.ContainsAny(line, "\x00") {
			isBinary = true
		}
		// If the file is binary and we've already matched something, we can
		// skip searching the rest of the file.
		if isBinary && !args.binaryAsText && (len(matchedLines) > 0) {
			break
		}
		matched = regex.Find(line)
		// If the "inverse" arg was specified, reverse the result so that non-
		// matching lines are counted as matching.
		if args.inverse {
			if matched == nil {
				matched = line
			} else {
				matched = nil
			}
		}
		if matched == nil {
			continue
		}
		if args.onlyMatched {
			matchedLines = append(matchedLines, matched)
		} else {
			matchedLines = append(matchedLines, line)
		}
	}
	if (e != nil) && (e != io.EOF) {
		return nil, false, fmt.Errorf("Error reading file: %s", e)
	}
	return matchedLines, isBinary, nil
}

// TODO: Scanning from stdin would be better if it were more pipelined.
func scanStdin(args *options, regex *regexp.Regexp) error {
	matchedLines, _, e := getFileMatches(args, regex, os.Stdin)
	if e != nil {
		return e
	}
	for _, line := range matchedLines {
		if args.hideFilenames {
			fmt.Printf("%s\n", line)
		} else {
			fmt.Printf("(standard input): %s\n", line)
		}
	}
	return nil
}

// This performs the scanning of each file, using the regular expression. Must
// be called after doDirectoryWalk.
func scanFiles(args *options) error {
	if args.insensitive {
		args.expression = "(?i)" + args.expression
	}
	regex, e := regexp.Compile(args.expression)
	if e != nil {
		return fmt.Errorf("Invalid expression: %s", e)
	}
	var matchedLines [][]byte
	isBinary := false
	var file *os.File
	// If no paths are given, then use stdin.
	if len(args.paths) == 0 {
		return scanStdin(args, regex)
	}
	isDir := false
	for _, path := range args.paths {
		isDir, e = isDirectory(path)
		if e != nil {
			return fmt.Errorf("Error checking if file is directory: %s\n", e)
		}
		if isDir {
			fmt.Printf("Directory: %s\n", path)
			continue
		}
		file, e = os.Open(path)
		if e != nil {
			return fmt.Errorf("Error opening file: %s", e)
		}
		matchedLines, isBinary, e = getFileMatches(args, regex, file)
		file.Close()
		if e != nil {
			return e
		}
		if len(matchedLines) == 0 {
			continue
		}
		if isBinary && !args.binaryAsText {
			fmt.Printf("Binary file %s matches.\n", path)
			continue
		}
		for _, line := range matchedLines {
			if args.hideFilenames {
				fmt.Printf("%s\n", line)
			} else {
				fmt.Printf("%s: %s\n", path, line)
			}
		}
	}
	return nil
}

func help() {
	fmt.Printf("This utility provides some of GNU grep's behavior.\n" +
		"Usage: grep [-irvahHo] <expression> [file paths]\n\n" +
		"  -r: If provided, recursively scan for files in the file paths\n" +
		"  -i: If provided, use case-insensitive matching\n" +
		"  -v: If provided, output lines which don't match\n" +
		"  -a: If provided, treat binary files as text\n" +
		"  -h: If provided, do not show filenames\n" +
		"  -H: If provided, always show filenames\n" +
		"  -o: If provided, only output the part of each line which matched\n")
	os.Exit(0)
}

func run() int {
	args, e := parseArgs()
	if e != nil {
		fmt.Printf("Failed parsing arguments: %s\n", e)
		fmt.Printf("Run with --help for more information.\n")
		return 1
	}
	e = doDirectoryWalk(args)
	if e != nil {
		fmt.Printf("Error recursively scanning files: %s\n", e)
		return 1
	}
	// Apply default behavior: omit filenames if only one file is being
	// scanned, unless -H is provided to always show filenames.
	if len(args.paths) <= 1 {
		if args.showFilenames {
			args.hideFilenames = false
		} else {
			args.hideFilenames = true
		}
	}
	e = scanFiles(args)
	if e != nil {
		fmt.Printf("%s\n", e)
		return 1
	}
	return 0
}

func main() {
	os.Exit(run())
}
