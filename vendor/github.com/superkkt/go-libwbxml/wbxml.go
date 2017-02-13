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

// #cgo pkg-config: libwbxml2
//
// #include <stdlib.h>
//
// extern int xml2wbxml(const char *xml, unsigned int xml_len, unsigned char **wbxml, unsigned int *wbxml_len);
// extern int wbxml2xml(const char *wbxml, unsigned int wbxml_len, unsigned char **xml, unsigned int *xml_len);
// extern const char *wbxml_error(int rc);
import "C"

import (
	"errors"
	"fmt"
	"unsafe"
)

func Encode(xml string) ([]byte, error) {
	if len(xml) == 0 {
		return nil, errors.New("empty XML string")
	}

	cXML := C.CString(xml)
	defer C.free(unsafe.Pointer(cXML))
	var wbxml *C.uchar
	var wbxmlLen C.uint

	ret := C.xml2wbxml(cXML, C.uint(len(xml)), &wbxml, &wbxmlLen)
	if ret != 0 {
		return nil, fmt.Errorf("Encode: %v", C.GoString(C.wbxml_error(ret)))
	}
	output := C.GoBytes(unsafe.Pointer(wbxml), C.int(wbxmlLen))
	C.free(unsafe.Pointer(wbxml))

	return output, nil
}

func Decode(wbxml []byte) (string, error) {
	if len(wbxml) == 0 {
		return "", errors.New("empty WBXML string")
	}

	cWBXML := C.CString(string(wbxml))
	defer C.free(unsafe.Pointer(cWBXML))
	var xml *C.uchar
	var xmlLen C.uint

	ret := C.wbxml2xml(cWBXML, C.uint(len(wbxml)), &xml, &xmlLen)
	if ret != 0 {
		return "", fmt.Errorf("Decode: %v", C.GoString(C.wbxml_error(ret)))
	}
	output := C.GoBytes(unsafe.Pointer(xml), C.int(xmlLen))
	C.free(unsafe.Pointer(xml))

	return string(output), nil
}
