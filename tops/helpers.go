package tops

import (
	. "github.com/ezBadminton/ezBadmintonServer/generated"
)

func idFinder[P Proxy, PP ProxyP[P]](id string) func(p PP) bool {
	return func(p PP) bool {
		return p.ProxyRecord().Id == id
	}
}
