package main

import (
	"flag"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
)

type options struct {
	Options
	// The directory where to write the output files.
	// This directory is usually relative to the `repopath`.
	// This is the value specified on the command line. It may be empty.
	outputPath string
	// The directory or file that should be built.
	buildPath string
	// A semi-colon separated list of pathes which define where to look for bundles.
	// Might be empty.
	searchPath string
	port       string
	server     bool
}

func main() {
	// Read command line options
	var options options
	var baseURL string
	flag.StringVar(&options.outputPath, "out", "", "Destination directory for the generated HTML, scripts, CSS, and images")
	flag.StringVar(&options.searchPath, "path", "", "Semi-colon separated list of directories that are searched for page types or bundles")
	flag.StringVar(&baseURL, "url", "/", "The (relative) URL to be used for the generated content")
	// flag.BoolVar(&options.server, "server", false, "Start the MaTeS server to be able to edit code on the fly")
	// flag.StringVar(&options.port, "port", "8080", "The port on which MaTeS server should listen for connections")
	flag.Parse()

	// Print usage if no file given
	if flag.NArg() != 1 {
		fmt.Fprintf(os.Stderr, "Usage: %s [OPTION] FILE_OR_DIRECTORY\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "\nOptions:\n")
		flag.PrintDefaults()
		return
	}

	// Check the provided URL
	var err error
	options.Options.BaseURL, err = url.Parse(baseURL)
	if err != nil {
		fmt.Fprintf(os.Stderr, "ERR: Malformed base URL %v", baseURL)
		return
	}

	// Append the environment variabale MATESPATH to the search path
	env := os.Getenv("MATESPATH")
	if env != "" {
		if options.searchPath == "" {
			options.searchPath = env
		} else {
			options.searchPath += string(filepath.ListSeparator) + env
		}
	}

	// Build all targets
	for _, arg := range flag.Args() {
		// Interpret arg relative to the current working directory
		cwd, _ := os.Getwd()
		arg = filepath.Clean(filepath.Join(cwd, arg))
		_, err = os.Stat(arg)
		if err != nil {
			fmt.Fprintf(os.Stderr, "ERR: Could not access file or directory %v\n", arg)
			return
		}
		// TODO: Reuse the same builder for files in the same directory for speedup
		options.buildPath = arg
		b, err := newBuilder(&options)
		if err != nil {
			fmt.Fprintf(os.Stderr, "ERR: %v\n", err)
			return
		}
		err = b.build()
		if err != nil {
			fmt.Fprintf(os.Stderr, "ERR: %v\n", err)
			return
		}
	}

	/*
		if options.server {
			server := NewServer(&files, &options)
			err := server.srv.ListenAndServe()
			if err != nil {
				panic(err)
			}
		}
	*/
}
