package irr

import (
	"bufio"
	"fmt"
	"io"
	"net"
	"strings"
)

type Route struct {
	Prefix *net.IPNet
	Origin string
}

func ParseRoutes(data io.Reader) ([]Route, error) {
	var routes []Route
	var route Route

	s := bufio.NewScanner(data)
	var line int
	for s.Scan() {
		line++
		f := strings.Fields(s.Text())

		// records are separated by a blank line
		if len(f) == 0 {
			// new record, but we did not finish the old record?
			if route.Prefix != nil || route.Origin != "" {
				return nil, fmt.Errorf("bad route: %+v, line: %d", route, line)
			}
			continue
		}

		switch strings.ToLower(f[0]) {
		case "route:", "route6:":
			if len(f) != 2 {
				return nil, fmt.Errorf("bad route: %s, line: %d", s.Text(), line)
			}
			_, cidr, err := net.ParseCIDR(f[1])
			if err != nil {
				return nil, fmt.Errorf("bad route: %s, line: %d", s.Text(), line)
			}
			route.Prefix = cidr
		case "origin:":
			if route.Prefix == nil {
				// old irrd servers used to change "route:" to "*xxte:" etc, ugly hack
				continue
			}
			if len(f) != 2 && !(len(f) > 2 && strings.HasPrefix(f[2], "#")) {
				return nil, fmt.Errorf("bad record: %s, line: %d", route.Prefix.String(), line)
			}
			route.Origin = f[1]
			routes = append(routes, route)
			route = Route{}
		}
	}

	if err := s.Err(); err != nil {
		return nil, err
	}

	return routes, nil
}
