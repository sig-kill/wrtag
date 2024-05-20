package main

import (
	"errors"
	"flag"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"slices"

	"go.senan.xyz/wrtag/cmd/internal/flags"
	"go.senan.xyz/wrtag/tags"
)

func init() {
	flag := flag.CommandLine
	flag.Usage = func() {
		fmt.Fprintf(flag.Output(), "Usage:\n")
		fmt.Fprintf(flag.Output(), "  $ %s [<options>] read  <tag>...                  -- <path>...\n", flag.Name())
		fmt.Fprintf(flag.Output(), "  $ %s [<options>] write ( <tag> <value>... , )... -- <path>...\n", flag.Name())
		fmt.Fprintf(flag.Output(), "  $ %s [<options>] clear <tag>...                  -- <path>...\n", flag.Name())
		fmt.Fprintf(flag.Output(), "\n")
		fmt.Fprintf(flag.Output(), "Example:\n")
		fmt.Fprintf(flag.Output(), "  $ %s read -- a.flac b.flac c.flac\n", flag.Name())
		fmt.Fprintf(flag.Output(), "  $ %s read artist title -- a.flac\n", flag.Name())
		fmt.Fprintf(flag.Output(), "  $ %s write album \"album name\" -- x.flac\n", flag.Name())
		fmt.Fprintf(flag.Output(), "  $ %s write genres \"psy\" \"minimal\" \"techno\" , artist \"Sensient\" -- dir/*.flac\n", flag.Name())
		fmt.Fprintf(flag.Output(), "  $ %s write genres \"psy\" \"minimal\" \"techno\" , artist \"Sensient\" -- dir/\n", flag.Name())
		fmt.Fprintf(flag.Output(), "  $ %s clear -- a.flac\n", flag.Name())
		fmt.Fprintf(flag.Output(), "  $ %s clear lyrics artist_credit -- *.flac\n", flag.Name())
		fmt.Fprintf(flag.Output(), "\n")
		fmt.Fprintf(flag.Output(), "Options:\n")
		flag.PrintDefaults()
	}
}

func main() {
	defer flags.ExitError()
	var (
		noProperties = flag.Bool("no-properties", false, "dont read file properties like length or bitrate")
	)
	flags.Parse()

	command := flag.Arg(0)

	switch command {
	case "read", "write", "clear":
	default:
		slog.Error("unknown command")
		return
	}

	argPaths := flag.Args()[1:]

	var args, paths []string
	if i := slices.Index(argPaths, "--"); i >= 0 {
		args = argPaths[:i]
		paths = argPaths[i+1:]
	}
	if len(paths) == 0 {
		slog.Error("no paths provided")
		return
	}

	var err error
	switch command {
	case "read":
		args := parseTags(args)
		err = iterFiles(paths, func(p string) error {
			return read(p, *noProperties, args)
		})
	case "write":
		args := parseTagMap(args)
		err = iterFiles(paths, func(p string) error {
			return write(p, args)
		})
	case "clear":
		args := parseTags(args)
		err = iterFiles(paths, func(p string) error {
			return clear(p, args)
		})
	}
	if err != nil {
		slog.Error("running", "err", err)
		return
	}
}

func read(path string, noProperties bool, keys map[string]struct{}) error {
	file, err := tags.Read(path)
	if err != nil {
		return fmt.Errorf("read: %w", err)
	}
	defer file.Close()

	wantKey := func(k string) bool {
		if len(keys) == 0 {
			return true
		}
		_, want := keys[k]
		return want
	}

	file.ReadAll(func(k string, vs []string) bool {
		if !wantKey(k) {
			return true
		}
		for _, v := range vs {
			fmt.Printf("%s\t%s\t%s\n", path, k, v)
		}
		return true
	})
	if noProperties {
		return nil
	}

	if k := "length"; wantKey(k) {
		fmt.Printf("%s\t%s\t%.2f\n", path, k, file.Length().Seconds())
	}
	if k := "bitrate"; wantKey(k) {
		fmt.Printf("%s\t%s\t%d\n", path, k, file.Bitrate())
	}
	if k := "sample_rate"; wantKey(k) {
		fmt.Printf("%s\t%s\t%d\n", path, k, file.SampleRate())
	}
	if k := "num_channels"; wantKey(k) {
		fmt.Printf("%s\t%s\t%d\n", path, k, file.NumChannels())
	}
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
			r[k] = nil
			continue
		}
		r[k] = append(r[k], v)
	}
	return r
}

func iterFiles(paths []string, f func(p string) error) error {
	var pathErrs []error
	for _, p := range paths {
		info, err := os.Stat(p)
		if err != nil {
			return err
		}

		switch info.Mode().Type() {
		// recurse if dir, only attempt when CanRead
		case os.ModeDir:
			err := filepath.WalkDir(p, func(path string, d fs.DirEntry, err error) error {
				if err != nil {
					return err
				}
				if !d.Type().IsRegular() {
					return nil
				}
				if !tags.CanRead(path) {
					return nil
				}
				if err := f(path); err != nil {
					pathErrs = append(pathErrs, err)
					return nil
				}
				return nil
			})
			if err != nil {
				return fmt.Errorf("walk: %w", err)
			}
		// otherwise try directly, bubble errors
		default:
			if err := f(p); err != nil {
				pathErrs = append(pathErrs, err)
				continue
			}
		}
	}
	return errors.Join(pathErrs...)
}
