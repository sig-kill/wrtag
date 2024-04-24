package musicbrainz

import (
	"strconv"
)

type StatusError int

func (se StatusError) Error() string {
	return strconv.Itoa(int(se))
}
