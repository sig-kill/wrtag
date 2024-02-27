package main

import (
	"bufio"
	"bytes"
	"context"
	_ "embed"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/rogpeppe/go-internal/testscript"
	"go.senan.xyz/wrtag/musicbrainz"
	"go.senan.xyz/wrtag/tags/tagcommon"
	"go.senan.xyz/wrtag/tags/taglib"
)

func TestMain(m *testing.M) {
	mb = &mockMB{
		// $ curl -s "https://musicbrainz.org/ws/2/release/XXX?fmt=json&inc=recordings%2Bartist-credits%2Blabels%2Brelease-groups"
		map[string]*musicbrainz.Release{
			"71d6f1d1-1190-4924-b2de-dfc1c2c8eea7": mustDecode[musicbrainz.Release]([]byte(`{"artist-credit":[{"name":"Alan Vega","artist":{"type-id":"b6e035f4-3ce9-331c-97df-83397230b0df","name":"Alan Vega","type":"Person","sort-name":"Vega, Alan","disambiguation":"","id":"dd720ac8-1c68-4484-abb7-0546413a55e3"},"joinphrase":""}],"status":null,"barcode":"3229261055827","packaging-id":null,"country":"FR","title":"Deuce Avenue","asin":null,"id":"71d6f1d1-1190-4924-b2de-dfc1c2c8eea7","label-info":[{"label":{"disambiguation":"French record label, at times styled as MUSIDISC with the Accord logo","type":"Original Production","id":"6fc14496-2cd1-40ec-b1b6-1a6efa7c83ac","sort-name":"Musidisc","name":"Musidisc","label-code":280,"type-id":"7aaa37fe-2def-3476-b359-80245850062d"},"catalog-number":"105582"}],"cover-art-archive":{"artwork":true,"count":1,"darkened":false,"back":false,"front":true},"release-events":[{"area":{"sort-name":"France","id":"08310658-51eb-3801-80de-5a0739207115","type-id":null,"name":"France","iso-3166-1-codes":["FR"],"type":null,"disambiguation":""},"date":"1990"}],"release-group":{"secondary-types":[],"artist-credit":[{"artist":{"id":"dd720ac8-1c68-4484-abb7-0546413a55e3","disambiguation":"","type":"Person","sort-name":"Vega, Alan","name":"Alan Vega","type-id":"b6e035f4-3ce9-331c-97df-83397230b0df"},"name":"Alan Vega","joinphrase":""}],"first-release-date":"1990","disambiguation":"","primary-type-id":"f529b476-6e62-324f-b0aa-1f3e33d313fc","title":"Deuce Avenue","secondary-type-ids":[],"primary-type":"Album","id":"0e3d4e08-7709-3c14-905e-1cb00c704066"},"disambiguation":"","packaging":null,"status-id":null,"quality":"normal","media":[{"format":"CD","tracks":[{"title":"Body Bop Jive","artist-credit":[{"joinphrase":"","name":"Alan Vega","artist":{"id":"dd720ac8-1c68-4484-abb7-0546413a55e3","disambiguation":"","type":"Person","sort-name":"Vega, Alan","name":"Alan Vega","type-id":"b6e035f4-3ce9-331c-97df-83397230b0df"}}],"recording":{"artist-credit":[{"joinphrase":"","name":"Alan Vega","artist":{"id":"dd720ac8-1c68-4484-abb7-0546413a55e3","disambiguation":"","sort-name":"Vega, Alan","type":"Person","name":"Alan Vega","type-id":"b6e035f4-3ce9-331c-97df-83397230b0df"}}],"first-release-date":"1990","disambiguation":"","title":"Body Bop Jive","video":false,"id":"f9d630a8-c195-4e3b-9445-3d24008339c1","length":283000},"id":"d336267e-5660-4c65-97fb-5655611db88a","number":"1","length":283000,"position":1},{"title":"Sneaker Gun Fire","artist-credit":[{"artist":{"type":"Person","sort-name":"Vega, Alan","disambiguation":"","id":"dd720ac8-1c68-4484-abb7-0546413a55e3","type-id":"b6e035f4-3ce9-331c-97df-83397230b0df","name":"Alan Vega"},"name":"Alan Vega","joinphrase":""}],"recording":{"length":320000,"id":"8f1188b6-aff1-4753-a6ff-ddba99fb0d0b","video":false,"title":"Sneaker Gun Fire","disambiguation":"","first-release-date":"1990","artist-credit":[{"artist":{"name":"Alan Vega","type-id":"b6e035f4-3ce9-331c-97df-83397230b0df","disambiguation":"","id":"dd720ac8-1c68-4484-abb7-0546413a55e3","type":"Person","sort-name":"Vega, Alan"},"name":"Alan Vega","joinphrase":""}]},"id":"0dac1f44-7379-4e8a-b266-c39ae4b7dc6b","number":"2","length":320000,"position":2},{"title":"Jab Gee","artist-credit":[{"artist":{"type-id":"b6e035f4-3ce9-331c-97df-83397230b0df","name":"Alan Vega","sort-name":"Vega, Alan","type":"Person","disambiguation":"","id":"dd720ac8-1c68-4484-abb7-0546413a55e3"},"name":"Alan Vega","joinphrase":""}],"recording":{"length":233000,"id":"c77f3ca6-9b66-49f1-915a-55a35b81a257","video":false,"title":"Jab Gee","disambiguation":"","first-release-date":"1990","artist-credit":[{"joinphrase":"","name":"Alan Vega","artist":{"name":"Alan Vega","type-id":"b6e035f4-3ce9-331c-97df-83397230b0df","id":"dd720ac8-1c68-4484-abb7-0546413a55e3","disambiguation":"","type":"Person","sort-name":"Vega, Alan"}}]},"id":"3bf4357f-f72f-45b4-8f39-cd9c38778f58","number":"3","length":233000,"position":3},{"artist-credit":[{"artist":{"type":"Person","sort-name":"Vega, Alan","disambiguation":"","id":"dd720ac8-1c68-4484-abb7-0546413a55e3","type-id":"b6e035f4-3ce9-331c-97df-83397230b0df","name":"Alan Vega"},"name":"Alan Vega","joinphrase":""}],"recording":{"artist-credit":[{"name":"Alan Vega","artist":{"type":"Person","sort-name":"Vega, Alan","disambiguation":"","id":"dd720ac8-1c68-4484-abb7-0546413a55e3","type-id":"b6e035f4-3ce9-331c-97df-83397230b0df","name":"Alan Vega"},"joinphrase":""}],"first-release-date":"1990","disambiguation":"","title":"Bad Scene","video":false,"length":240000,"id":"ca2bc8b6-b98a-4ac6-a9ed-b512e9d4eeb3"},"title":"Bad Scene","position":4,"id":"54df089d-1490-465c-82d1-c8d6cc5318b0","number":"4","length":240000},{"recording":{"video":false,"length":250000,"id":"2d943450-1faf-41c4-8079-65c96d80c35d","title":"La La Bola","first-release-date":"1990","disambiguation":"","artist-credit":[{"name":"Alan Vega","artist":{"name":"Alan Vega","type-id":"b6e035f4-3ce9-331c-97df-83397230b0df","id":"dd720ac8-1c68-4484-abb7-0546413a55e3","disambiguation":"","sort-name":"Vega, Alan","type":"Person"},"joinphrase":""}]},"artist-credit":[{"joinphrase":"","artist":{"sort-name":"Vega, Alan","type":"Person","id":"dd720ac8-1c68-4484-abb7-0546413a55e3","disambiguation":"","type-id":"b6e035f4-3ce9-331c-97df-83397230b0df","name":"Alan Vega"},"name":"Alan Vega"}],"title":"La La Bola","position":5,"number":"5","length":250000,"id":"30922e4a-f71d-443f-b4e6-c3967c972857"},{"title":"Deuce Avenue","recording":{"length":240000,"id":"c632bb0e-2cda-4ab9-afe1-9ef1e20d6c9a","video":false,"title":"Deuce Avenue","disambiguation":"","first-release-date":"1990","artist-credit":[{"joinphrase":"","artist":{"id":"dd720ac8-1c68-4484-abb7-0546413a55e3","disambiguation":"","type":"Person","sort-name":"Vega, Alan","name":"Alan Vega","type-id":"b6e035f4-3ce9-331c-97df-83397230b0df"},"name":"Alan Vega"}]},"artist-credit":[{"artist":{"type":"Person","sort-name":"Vega, Alan","id":"dd720ac8-1c68-4484-abb7-0546413a55e3","disambiguation":"","type-id":"b6e035f4-3ce9-331c-97df-83397230b0df","name":"Alan Vega"},"name":"Alan Vega","joinphrase":""}],"number":"6","length":240000,"id":"f9af0a78-66c4-4095-84bd-fabc3f4108e2","position":6},{"position":7,"length":220000,"number":"7","id":"ad3ad0a9-8e92-48d8-9ff3-ad4bf02ae147","recording":{"title":"Faster Blaster","id":"8c3cfe72-a438-4562-85f7-9c987123649f","length":220000,"video":false,"artist-credit":[{"joinphrase":"","name":"Alan Vega","artist":{"type-id":"b6e035f4-3ce9-331c-97df-83397230b0df","name":"Alan Vega","type":"Person","sort-name":"Vega, Alan","disambiguation":"","id":"dd720ac8-1c68-4484-abb7-0546413a55e3"}}],"disambiguation":"","first-release-date":"1990"},"artist-credit":[{"joinphrase":"","artist":{"disambiguation":"","id":"dd720ac8-1c68-4484-abb7-0546413a55e3","sort-name":"Vega, Alan","type":"Person","name":"Alan Vega","type-id":"b6e035f4-3ce9-331c-97df-83397230b0df"},"name":"Alan Vega"}],"title":"Faster Blaster"},{"title":"Sugee","artist-credit":[{"joinphrase":"","artist":{"id":"dd720ac8-1c68-4484-abb7-0546413a55e3","disambiguation":"","sort-name":"Vega, Alan","type":"Person","name":"Alan Vega","type-id":"b6e035f4-3ce9-331c-97df-83397230b0df"},"name":"Alan Vega"}],"recording":{"video":false,"id":"dcd2d93e-d195-48dc-8442-630e9402c646","length":265000,"title":"Sugee","first-release-date":"1990","disambiguation":"","artist-credit":[{"joinphrase":"","artist":{"disambiguation":"","id":"dd720ac8-1c68-4484-abb7-0546413a55e3","sort-name":"Vega, Alan","type":"Person","name":"Alan Vega","type-id":"b6e035f4-3ce9-331c-97df-83397230b0df"},"name":"Alan Vega"}]},"id":"0994b7ea-bfc8-4ba4-a30c-d9b56c2be7d4","length":265000,"number":"8","position":8},{"number":"9","length":253000,"id":"ab867b28-dc24-4d5e-b174-f4ed876486b4","position":9,"title":"Sweet Sweet Money","recording":{"artist-credit":[{"joinphrase":"","artist":{"type-id":"b6e035f4-3ce9-331c-97df-83397230b0df","name":"Alan Vega","type":"Person","sort-name":"Vega, Alan","id":"dd720ac8-1c68-4484-abb7-0546413a55e3","disambiguation":""},"name":"Alan Vega"}],"first-release-date":"1990","disambiguation":"","title":"Sweet Sweet Money","video":false,"length":253000,"id":"2fdb5c11-507f-4dec-9009-429688204a61"},"artist-credit":[{"name":"Alan Vega","artist":{"type-id":"b6e035f4-3ce9-331c-97df-83397230b0df","name":"Alan Vega","type":"Person","sort-name":"Vega, Alan","disambiguation":"","id":"dd720ac8-1c68-4484-abb7-0546413a55e3"},"joinphrase":""}]},{"recording":{"title":"Love On","video":false,"length":270000,"id":"8efef622-c866-4b70-adf4-1487ffd7ad84","artist-credit":[{"joinphrase":"","name":"Alan Vega","artist":{"type":"Person","sort-name":"Vega, Alan","disambiguation":"","id":"dd720ac8-1c68-4484-abb7-0546413a55e3","type-id":"b6e035f4-3ce9-331c-97df-83397230b0df","name":"Alan Vega"}}],"first-release-date":"1990","disambiguation":""},"artist-credit":[{"artist":{"sort-name":"Vega, Alan","type":"Person","disambiguation":"","id":"dd720ac8-1c68-4484-abb7-0546413a55e3","type-id":"b6e035f4-3ce9-331c-97df-83397230b0df","name":"Alan Vega"},"name":"Alan Vega","joinphrase":""}],"title":"Love On","position":10,"length":270000,"number":"10","id":"c8232991-ed15-4e22-ad3d-67f275d23abb"},{"title":"No Tomorrow","recording":{"first-release-date":"1990","disambiguation":"","artist-credit":[{"joinphrase":"","name":"Alan Vega","artist":{"sort-name":"Vega, Alan","type":"Person","disambiguation":"","id":"dd720ac8-1c68-4484-abb7-0546413a55e3","type-id":"b6e035f4-3ce9-331c-97df-83397230b0df","name":"Alan Vega"}}],"video":false,"length":265000,"id":"99995871-103f-4754-bb29-6d92a4dcc2f2","title":"No Tomorrow"},"artist-credit":[{"artist":{"disambiguation":"","id":"dd720ac8-1c68-4484-abb7-0546413a55e3","type":"Person","sort-name":"Vega, Alan","name":"Alan Vega","type-id":"b6e035f4-3ce9-331c-97df-83397230b0df"},"name":"Alan Vega","joinphrase":""}],"number":"11","length":265000,"id":"8818d325-d370-4160-8f37-d15e894d8141","position":11},{"id":"be46da0e-15af-4a46-99bd-78da4cc0ca95","number":"12","length":300000,"position":12,"title":"Future Sex","artist-credit":[{"joinphrase":"","artist":{"type":"Person","sort-name":"Vega, Alan","id":"dd720ac8-1c68-4484-abb7-0546413a55e3","disambiguation":"","type-id":"b6e035f4-3ce9-331c-97df-83397230b0df","name":"Alan Vega"},"name":"Alan Vega"}],"recording":{"title":"Future Sex","length":300000,"id":"58838d33-7651-4ac8-82ca-cb783aaff17f","video":false,"artist-credit":[{"name":"Alan Vega","artist":{"sort-name":"Vega, Alan","type":"Person","disambiguation":"","id":"dd720ac8-1c68-4484-abb7-0546413a55e3","type-id":"b6e035f4-3ce9-331c-97df-83397230b0df","name":"Alan Vega"},"joinphrase":""}],"disambiguation":"","first-release-date":"1990"}}],"position":1,"format-id":"9712d52a-4509-3d4b-a1a2-67c88c643e31","track-offset":0,"track-count":12,"title":""}],"text-representation":{"language":null,"script":null},"date":"1990"}`)),
			"e47d04a4-7460-427d-a731-cc82386d85f1": mustDecode[musicbrainz.Release]([]byte(`{"artist-credit":[{"artist":{"type":"Person","disambiguation":"Detroit based DJ","id":"470a4ced-1323-4c91-8fd5-0bb3fb4c932a","name":"Jeff Mills","type-id":"b6e035f4-3ce9-331c-97df-83397230b0df","sort-name":"Mills, Jeff"},"joinphrase":"","name":"Jeff Mills"}],"text-representation":{"script":"Latn","language":"eng"},"label-info":[{"catalog-number":"PMD002","label":{"type-id":"7aaa37fe-2def-3476-b359-80245850062d","sort-name":"Purpose Maker","disambiguation":"","label-code":null,"name":"Purpose Maker","id":"f7a74ee5-6e48-4767-9351-9cde838ec6a7","type":"Original Production"}}],"country":"XW","status-id":"4e304316-386d-3409-af2e-78857eec5cfe","status":"Official","release-group":{"first-release-date":"1997","artist-credit":[{"joinphrase":"","name":"Jeff Mills","artist":{"type":"Person","sort-name":"Mills, Jeff","type-id":"b6e035f4-3ce9-331c-97df-83397230b0df","id":"470a4ced-1323-4c91-8fd5-0bb3fb4c932a","name":"Jeff Mills","disambiguation":"Detroit based DJ"}}],"disambiguation":"","primary-type":"EP","secondary-types":[],"secondary-type-ids":[],"id":"acb38b21-9063-3ea3-b578-35c14d9aa488","title":"Kat Moda EP","primary-type-id":"6d0c5bf6-7a33-3420-a519-44fc63eedebf"},"packaging-id":"119eba76-b343-3e02-a292-f0f00644bb9b","release-events":[{"date":"","area":{"iso-3166-1-codes":["XW"],"type":null,"id":"525d4e18-3d00-31b9-a58b-a146a916de8f","name":"[Worldwide]","disambiguation":"","sort-name":"[Worldwide]","type-id":null}}],"title":"Kat Moda","media":[{"tracks":[{"artist-credit":[{"artist":{"sort-name":"Mills, Jeff","type-id":"b6e035f4-3ce9-331c-97df-83397230b0df","name":"Jeff Mills","id":"470a4ced-1323-4c91-8fd5-0bb3fb4c932a","disambiguation":"Detroit based DJ","type":"Person"},"name":"Jeff Mills","joinphrase":""}],"recording":{"first-release-date":"1997","artist-credit":[{"artist":{"type":"Person","name":"Jeff Mills","id":"470a4ced-1323-4c91-8fd5-0bb3fb4c932a","disambiguation":"Detroit based DJ","sort-name":"Mills, Jeff","type-id":"b6e035f4-3ce9-331c-97df-83397230b0df"},"joinphrase":"","name":"Jeff Mills"}],"disambiguation":"","id":"93b7876b-c37d-4d42-8b8e-083250e6a8a3","length":317933,"video":false,"title":"Alarms"},"position":1,"length":317933,"title":"Alarms","number":"1","id":"084e4019-8d64-4f9f-b1a3-d4459d8a5829"},{"title":"The Bells","length":292880,"position":2,"number":"2","id":"da9a42ca-27e0-4279-9473-23fb033c9fd8","artist-credit":[{"name":"Jeff Mills","joinphrase":"","artist":{"name":"Jeff Mills","id":"470a4ced-1323-4c91-8fd5-0bb3fb4c932a","disambiguation":"Detroit based DJ","sort-name":"Mills, Jeff","type-id":"b6e035f4-3ce9-331c-97df-83397230b0df","type":"Person"}}],"recording":{"id":"a8ea2c29-1c4b-456d-a977-19497a11f0a8","disambiguation":"","title":"The Bells","video":false,"length":287453,"first-release-date":"1996","artist-credit":[{"joinphrase":"","name":"Jeff Mills","artist":{"disambiguation":"Detroit based DJ","id":"470a4ced-1323-4c91-8fd5-0bb3fb4c932a","name":"Jeff Mills","type-id":"b6e035f4-3ce9-331c-97df-83397230b0df","sort-name":"Mills, Jeff","type":"Person"}}]}},{"title":"The Bells (Festival mix)","length":606866,"position":3,"number":"3","id":"7ccbc644-014c-4c5a-9cb0-eb0bb895bf7a","artist-credit":[{"joinphrase":"","name":"Jeff Mills","artist":{"type":"Person","sort-name":"Mills, Jeff","type-id":"b6e035f4-3ce9-331c-97df-83397230b0df","name":"Jeff Mills","id":"470a4ced-1323-4c91-8fd5-0bb3fb4c932a","disambiguation":"Detroit based DJ"}}],"recording":{"length":606866,"title":"The Bells (Festival mix)","video":false,"disambiguation":"","id":"a5327233-aa63-4b25-9ac4-a18cf35704a8","artist-credit":[{"name":"Jeff Mills","joinphrase":"","artist":{"type-id":"b6e035f4-3ce9-331c-97df-83397230b0df","sort-name":"Mills, Jeff","disambiguation":"Detroit based DJ","id":"470a4ced-1323-4c91-8fd5-0bb3fb4c932a","name":"Jeff Mills","type":"Person"}}]}}],"position":1,"track-offset":0,"title":"","track-count":3,"format-id":"907a28d9-b3b2-3ef6-89a8-7b18d91d4794","format":"Digital Media"}],"asin":null,"cover-art-archive":{"back":false,"count":1,"darkened":false,"front":true,"artwork":true},"disambiguation":"","quality":"normal","date":"","id":"e47d04a4-7460-427d-a731-cc82386d85f1","barcode":null,"packaging":"None"}`)),
		},
	}

	os.Exit(testscript.RunMain(m, map[string]func() int{
		"wrtag":     func() int { main(); return 0 },
		"gen-files": func() int { mainGenAudioFiles(); return 0 },
		"find":      func() int { mainFind(); return 0 },
	}))
}

func TestScripts(t *testing.T) {
	testscript.Run(t, testscript.Params{
		Dir:                 "testdata",
		RequireExplicitExec: true,
	})
}

type mockMB struct {
	releases map[string]*musicbrainz.Release
}

func (c *mockMB) SearchRelease(ctx context.Context, q musicbrainz.ReleaseQuery) (*musicbrainz.Release, error) {
	if r, ok := c.releases[q.MBReleaseID]; ok {
		return r, nil
	}
	return nil, musicbrainz.ErrNoResults
}

func mainGenAudioFiles() {
	flag.Parse()

	var tg tagcommon.Reader = taglib.TagLib{}

	paths, err := filepath.Glob(flag.Arg(0))
	if err != nil {
		log.Fatalln("glob paths: %w", err)
	}

	var pathErrs []error
	for _, p := range paths {
		pathErrs = append(pathErrs, genAudioFile(tg, p))
	}
	if err := errors.Join(pathErrs...); err != nil {
		log.Fatalf("process file: %v", err)
	}
}

//go:embed testdata/empty.flac
var emptyFlac []byte

func genAudioFile(tg tagcommon.Reader, path string) error {
	tmpl, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read tmpl: %w", err)
	}

	// replace with empty flac
	emptyFile, err := os.OpenFile(path, os.O_WRONLY|os.O_TRUNC, 0)
	if err != nil {
		return fmt.Errorf("open and trunc file: %w", err)
	}
	emptyFile.Write(emptyFlac)
	emptyFile.Close()

	f, err := tg.Read(path)
	if err != nil {
		return fmt.Errorf("open tag file: %w", err)
	}
	defer f.Close()

	var jsonErrors []error
	for sc := bufio.NewScanner(bytes.NewReader(tmpl)); sc.Scan(); {
		k, v, ok := strings.Cut(sc.Text(), " ")
		if !ok {
			continue
		}

		k = strings.TrimSpace(k)
		method := reflect.ValueOf(f).MethodByName(k)
		dest := reflect.New(method.Type().In(0))
		if err := json.Unmarshal([]byte(v), dest.Interface()); err != nil {
			jsonErrors = append(jsonErrors, err)
			continue
		}

		method.Call([]reflect.Value{dest.Elem()})
	}

	return errors.Join(jsonErrors...)
}

func mainFind() {
	flag.Parse()

	paths := flag.Args()
	sort.Strings(paths)

	for _, p := range paths {
		filepath.WalkDir(p, func(path string, d fs.DirEntry, err error) error {
			fmt.Println(path)
			return nil
		})
	}
}

func mustDecode[T any](data []byte) *T {
	var t T
	if err := json.Unmarshal(data, &t); err != nil {
		panic(err)
	}
	return &t
}
