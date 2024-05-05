package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"slices"

	"go.senan.xyz/wrtag/tags"
)

func main() {
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "usage:\n")
		fmt.Fprintf(os.Stderr, "  $ %s read  [TAG]...               -- [PATH]...\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "  $ %s write [TAG [VALUE]... , ]... -- [PATH]...\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "  $ %s clear [TAG]...               -- [PATH]...\n", os.Args[0])
		fmt.Fprintln(os.Stderr)
		fmt.Fprintf(os.Stderr, "example:\n")
		fmt.Fprintf(os.Stderr, "  $ %s read -- a.flac b.flac c.flac\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "  $ %s read artist title -- a.flac\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "  $ %s write album \"album name\" -- x.flac\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "  $ %s write genres \"psy\" \"minimal\" \"techno\" , artist \"Sensient\" -- dir/*.flac\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "  $ %s clear -- a.flac\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "  $ %s clear lyrics artist_credit -- *.flac\n", os.Args[0])
	}
	flag.Parse()

	command := flag.Arg(0)

	switch command {
	case "read", "write", "clear":
	default:
		flag.Usage()
		os.Exit(1)
	}

	argPaths := flag.Args()[1:]

	var args, paths []string
	if i := slices.Index(argPaths, "--"); i >= 0 {
		args = argPaths[:i]
		paths = argPaths[i+1:]
	}
	if len(paths) == 0 {
		fmt.Fprintf(os.Stderr, "no paths provided\n")
		fmt.Fprintln(os.Stderr)
		flag.Usage()
		os.Exit(1)
	}

	var errs []error
	switch command {
	case "read":
		args := parseTags(args)
		for _, path := range paths {
			if err := read(path, args); err != nil {
				errs = append(errs, err)
				continue
			}
		}
	case "write":
		args := parseTagMap(args)
		for _, path := range paths {
			if err := write(path, args); err != nil {
				errs = append(errs, err)
				continue
			}
		}
	case "clear":
		args := parseTags(args)
		for _, path := range paths {
			if err := clear(path, args); err != nil {
				errs = append(errs, err)
				continue
			}
		}
	}
	if err := errors.Join(errs...); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func read(path string, keys map[string]struct{}) error {
	file, err := tags.Read(path)
	if err != nil {
		return fmt.Errorf("read: %w", err)
	}
	defer file.Close()
	file.ReadAll(func(k string, vs []string) bool {
		if len(keys) > 0 {
			if _, ok := keys[k]; !ok {
				return true
			}
		}
		for _, v := range vs {
			fmt.Printf("%s\t%s\t%s\n", path, k, v)
		}
		return true
	})
	return nil
}

func write(path string, raw map[string][]string) error {
	file, err := tags.Read(path)
	if err != nil {
		return fmt.Errorf("read: %w", err)
	}
	defer file.Close()
	for k, vs := range raw {
		file.Write(k, vs...)
	}
	if err := file.Save(); err != nil {
		return fmt.Errorf("save: %w", err)
	}
	return nil
}

func clear(path string, keys map[string]struct{}) error {
	file, err := tags.Read(path)
	if err != nil {
		return fmt.Errorf("read: %w", err)
	}
	defer file.Close()
	if len(keys) == 0 {
		file.ClearAll()
	} else {
		for k := range keys {
			file.Clear(k)
		}
	}
	if err := file.Save(); err != nil {
		return fmt.Errorf("save: %w", err)
	}
	return nil
}

func parseTags(args []string) map[string]struct{} {
	var keys = map[string]struct{}{}
	for _, k := range args {
		keys[k] = struct{}{}
	}
	return keys
}

func parseTagMap(args []string) map[string][]string {
	r := make(map[string][]string)
	var k string
	for _, v := range args {
		if v == "," {
			k = ""
			continue
		}
		if k == "" {
			k = v
			continue
		}
		r[k] = append(r[k], v)
	}
	return r
}
