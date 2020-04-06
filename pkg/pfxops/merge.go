package pfxops

import (
	"fmt"
	"net"

	"github.com/dotwaffle/ovplusplus/pkg/irr"
	"github.com/dotwaffle/ovplusplus/pkg/rpki"
	"github.com/yl2chen/cidranger"
)

type pfxAsn struct {
	prefix string
	asn    string
}

func merge(roas []rpki.ROA, irrdb map[string][]irr.Route, unsafe bool) ([]rpki.ROA, error) {
	pfxMap := make(map[pfxAsn]bool)
	for _, roa := range roas {
		pfxMap[pfxAsn{prefix: roa.Prefix, asn: roa.ASN}] = true
	}

	pfxTrie := cidranger.NewPCTrieRanger()
	for _, roa := range roas {
		_, cidr, err := net.ParseCIDR(roa.Prefix)
		if err != nil {
			return nil, fmt.Errorf("bad prefix: %s: %w", roa.Prefix, err)
		}
		pfxTrie.Insert(cidranger.NewBasicRangerEntry(*cidr))
	}

	for db, routes := range irrdb {
		for _, route := range routes {
			routeStr := route.Prefix.String()

			if _, ok := pfxMap[pfxAsn{prefix: routeStr, asn: route.Origin}]; ok {
				// already seen this Prefix/ASN combination, skip
				continue
			}
			pfxMap[pfxAsn{prefix: routeStr, asn: route.Origin}] = true

			// does an ROA already cover this prefix, either with a shorter or longer prefix
			roa, err := pfxTrie.Contains(route.Prefix.IP)
			if err != nil {
				return nil, fmt.Errorf("pfxTrie.Contains(): %s: %w", routeStr, err)
			}

			// no ROA covers this prefix or unsafe mode active
			if !roa || unsafe {
				ones, _ := route.Prefix.Mask.Size()
				newROA := rpki.ROA{
					Prefix:    routeStr,
					MaxLength: ones,
					ASN:       route.Origin,
					TA:        db,
				}
				roas = append(roas, newROA)
			}
		}
	}

	return roas, nil
}

func Merge(roas []rpki.ROA, irrdb map[string][]irr.Route) ([]rpki.ROA, error) {
	return merge(roas, irrdb, false)
}

func MergeUnsafe(roas []rpki.ROA, irrdb map[string][]irr.Route) ([]rpki.ROA, error) {
	return merge(roas, irrdb, true)
}
