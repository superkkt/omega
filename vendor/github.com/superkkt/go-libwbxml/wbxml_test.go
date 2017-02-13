/*
 * Go-binding of libwbxml
 *
 * Copyright 2016 Samjung Data Service, Inc. All Rights Reserved.
 *
 * Authors:
 *      Kitae Kim <superkkt@sds.co.kr>
 *      Sungsoon Lim <sungsoon0813@sds.co.kr>
 */

package wbxml

import (
	"fmt"
	"testing"
)

const (
	xml = `<?xml version="1.0" encoding="utf-8"?><Sync xmlns="AirSync" xmlns:email="Email"><Collections><Collection><CollectionId>1</CollectionId><Status>1</Status></Collection></Collections></Sync>`
)

func ExampleEncode() {
	v, err := Encode(xml)
	if err != nil {
		panic(err)
	}
	fmt.Println(v)
	// Output: [3 1 106 0 69 92 79 82 3 49 0 1 78 3 49 0 1 1 1 1]
}

func ExampleDecode() {
	v, err := Decode([]byte{3, 1, 106, 0, 69, 92, 79, 82, 3, 49, 0, 1, 78, 3, 49, 0, 1, 1, 1, 1})
	if err != nil {
		panic(err)
	}
	fmt.Println(v)
	// Output: <?xml version="1.0"?><!DOCTYPE ActiveSync PUBLIC "-//MICROSOFT//DTD ActiveSync//EN" "http://www.microsoft.com/"><Sync xmlns="AirSync:"><Collections><Collection><CollectionId>1</CollectionId><Status>1</Status></Collection></Collections></Sync>
}

func TestInvalidXML(t *testing.T) {
	_, err := Encode("abcdef")
	if err == nil {
		t.Fatal("we expect an error, but we got nil error!")
	}

	_, err = Decode([]byte{0, 0, 0})
	if err == nil {
		t.Fatal("we expect an error, but we got nil error!")
	}
}
