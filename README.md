Cross-platform grep implementation
==================================

This project sought to be a cross-platform utility compatible with some basic
features of GNU grep. It is written in Go so that it can be easily built for
Windows, but at the cost of GNU grep's excellent performance.

It currently supports a few of the short command-line flags from GNU grep,
and uses Go's regular expressions.

Compiling / Installation
------------------------

For simple installation, just run `go install github.com/yalue/grep`.
Alternatively, download links for Windows can be found on [the releases
page](https://github.com/yalue/grep/releases).

Usage
-----

Basic usage: `grep [flags] <expression> [file paths]`. If no file paths are
provided, the program will read from standard input.

The regular expression supports Go's regular expression syntax. The file path
accepts wildcards, and will be expanded according to Go's `filepath.Glob()`.
Running the program with the `--help` argument will display usage information.
The following flags are supported, with behavior identical to GNU grep:

 - `-i`: Perform case-insensitive matching.

 - `-r`: Recursively scan files in all directories matching the given filepaths.

 - `-v`: Print the inverse of each match; don't include lines which did match.

 - `-o`: Print only the portion of each line which matched the regular
   expression. The default behavior is to print the entire matching line.

 - `-h`: Never show filenames for matching lines. This is the default if only
   one file (or standard input) is being scanned.

 - `-H`: Always show filenames for matching lines. This is the default if more
   than one file is being scanned.

 - `-a`: Treat binary files as text files. The default behavior will simply
   print a message if a matching line is found in a binary file. Any file
   containing a null byte is counted as "binary".

