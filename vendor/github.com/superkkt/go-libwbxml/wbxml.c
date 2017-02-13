/*
 * Go-binding of libwbxml
 *
 * Copyright 2016 Samjung Data Service, Inc. All Rights Reserved.
 *
 * Authors:
 *      Kitae Kim <superkkt@sds.co.kr>
 *      Sungsoon Lim <sungsoon0813@sds.co.kr>
 */

#include <wbxml/wbxml.h>

// wbxml should be freed by the caller.
int xml2wbxml(const char *xml, unsigned int xml_len, unsigned char **wbxml, unsigned int *wbxml_len) {
	WBXMLConvXML2WBXML *enc = NULL;
	WBXMLError ret = WBXML_OK;

	if ((ret = wbxml_conv_xml2wbxml_create(&enc)) != WBXML_OK) {
		goto cleanup;
	}
	// Do not add public document type in the encoded WBXML.
	wbxml_conv_xml2wbxml_disable_public_id(enc);
	// Do not use string tables because ActiveSync does not use it!
	wbxml_conv_xml2wbxml_disable_string_table(enc);
	// Do not ignore white spaces.
	wbxml_conv_xml2wbxml_enable_preserve_whitespaces(enc);
	if ((ret = wbxml_conv_xml2wbxml_run(enc, (WB_UTINY *) xml, xml_len, wbxml, wbxml_len)) != WBXML_OK) {
		goto cleanup;
	}

cleanup:
	if (enc != NULL) {
		wbxml_conv_xml2wbxml_destroy(enc);
	}

	return ret;
}

// xml should be freed by the caller.
int wbxml2xml(const char *wbxml, unsigned int wbxml_len, unsigned char **xml, unsigned int *xml_len) {
	WBXMLConvWBXML2XML *dec = NULL;
	WBXMLError ret = WBXML_OK;

	if ((ret = wbxml_conv_wbxml2xml_create(&dec)) != WBXML_OK) {
		goto cleanup;
	}
	// Use ActiveSync code page even if there is no public document type in the encoded WBXML.
	wbxml_conv_wbxml2xml_set_language(dec, WBXML_LANG_ACTIVESYNC);
	wbxml_conv_wbxml2xml_set_gen_type(dec, WBXML_GEN_XML_COMPACT);
	wbxml_conv_wbxml2xml_set_charset(dec, WBXML_CHARSET_UTF_8);
	// Do not ignore white spaces.
	wbxml_conv_wbxml2xml_enable_preserve_whitespaces(dec);
	if ((ret = wbxml_conv_wbxml2xml_run(dec, (WB_UTINY *) wbxml, wbxml_len, xml, xml_len)) != WBXML_OK) {
		goto cleanup;
	}

cleanup:
	if (dec != NULL) {
		wbxml_conv_wbxml2xml_destroy(dec);
	}

	return ret;
}

const char *wbxml_error(int rc) {
	return (const char *) wbxml_errors_string(rc);
}
