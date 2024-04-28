package musicbrainz

import (
	"strconv"
)

var _ error = StatusError(0)

type StatusError int

func (se StatusError) Error() string {
	return strconv.Itoa(int(se))
}
