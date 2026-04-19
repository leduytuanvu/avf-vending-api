package httpserver

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"
)

const (
	adminListLimitDefault int32 = 50
	adminListLimitMax     int32 = 500
)

func parseAdminLimitOffset(r *http.Request) (limit int32, offset int32, err error) {
	rawLimit := strings.TrimSpace(r.URL.Query().Get("limit"))
	if rawLimit == "" {
		limit = adminListLimitDefault
	} else {
		n, perr := strconv.ParseInt(rawLimit, 10, 32)
		if perr != nil || n <= 0 {
			return 0, 0, fmt.Errorf("limit must be a positive integer")
		}
		if n > int64(adminListLimitMax) {
			limit = adminListLimitMax
		} else {
			limit = int32(n)
		}
	}
	rawOff := strings.TrimSpace(r.URL.Query().Get("offset"))
	if rawOff == "" {
		return limit, 0, nil
	}
	o, perr := strconv.ParseInt(rawOff, 10, 32)
	if perr != nil || o < 0 {
		return 0, 0, fmt.Errorf("offset must be a non-negative integer")
	}
	return limit, int32(o), nil
}
