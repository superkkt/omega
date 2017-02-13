package backend

import (
	"net/mail"
	"strings"

	"github.com/Muzikatoshi/omega/backend"
)

func parseAddress(address string) backend.EmailAddress {
	v, err := mail.ParseAddress(address)
	if err != nil {
		// fallback
		return backend.EmailAddress{
			Name:    "",
			Address: address,
		}
	}

	return backend.EmailAddress{
		Name:    v.Name,
		Address: v.Address,
	}
}

func parseAddressList(address string) []backend.EmailAddress {
	arr := strings.Split(address, ",")

	addrs := make([]backend.EmailAddress, len(arr))
	for i, v := range arr {
		addrs[i] = parseAddress(v)
	}

	return addrs
}
