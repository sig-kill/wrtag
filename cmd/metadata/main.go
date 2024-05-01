package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"

	"github.com/sentriz/audiotags"
)

func main() {
	flag.Parse()

	var errs []error
	for _, path := range flag.Args() {
		f, err := audiotags.Open(path)
		if err != nil {
			errs = append(errs, fmt.Errorf("read %s: %w", path, err))
			continue
		}

		props := f.ReadAudioProperties()
		raw := f.ReadTags()

		for k, vs := range raw {
			fmt.Printf("%s\t%s\t%s\n", path, k, enc(vs))
		}
		fmt.Printf("%s\t%s\t%s\n", path, "bitrate", enc(props.Bitrate))
		fmt.Printf("%s\t%s\t%s\n", path, "length", enc(props.Length))

		f.Close()
	}

	if err := errors.Join(errs...); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func enc(v any) string {
	r, _ := json.Marshal(v)
	return string(r)
}
