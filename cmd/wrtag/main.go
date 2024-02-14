package main

import (
	"go.senan.xyz/wrtag/cmd/internal/conf"

	"go.senan.xyz/wrtag/musicbrainz"
	"go.senan.xyz/wrtag/tags/taglib"
)

func main() {
	conf.Parse()

	mb := musicbrainz.NewClient()
	tg := taglib.TagLib{}

	_ = mb
	_ = tg

	// for _, dir := range ffs.GetArgs() {
	// 	_ = dir
	// 	if err := processJob(context.Background(), mb, tg, pathFormat, nil); err != nil {
	// 		log.Printf("error processing dir %q: %v", dir, err)
	// 		continue
	// 	}
	// }
}
