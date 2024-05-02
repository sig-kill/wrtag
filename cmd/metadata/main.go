package main

import (
	"errors"
	"flag"
	"fmt"
	"os"

	"go.senan.xyz/wrtag/tags"
)

func main() {
	flag.Parse()

	var errs []error
	for _, path := range flag.Args() {
		file, err := tags.Read(path)
		if err != nil {
			errs = append(errs, fmt.Errorf("read %s: %w", path, err))
			continue
		}
		file.ReadAll(func(k string, vs []string) {
			for _, v := range vs {
				fmt.Printf("%s\t%s\t%s\n", path, k, v)
			}
		})
		file.Close()
	}

	if err := errors.Join(errs...); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
