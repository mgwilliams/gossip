package gossip

import "bytes"
import "fmt"
import "strconv"
import "strings"

// A single logical header from a SIP message.
type SipHeader interface {
	// Produce the string representation of the header.
	String() string
}

// A URI from any schema (e.g. sip:, tel:, callto:)
type Uri interface {
	// Determine if the two URIs are equal according to the rules in RFC 3261 s. 19.1.4.
	Equals(other Uri) bool

	// Produce the string representation of the URI.
	String() string
}

// A URI from a schema suitable for inclusion in a Contact: header.
// The only such URIs are sip/sips URIs and the special wildcard URI '*'.
type ContactUri interface {
	// Determine if the two URIs are equal according to the rules in RFC 3261 s. 19.1.4.
	Equals(other Uri) bool

	// Produce the string representation of the URI.
	String() string

	// Return true if and only if the URI is the special wildcard URI '*'; that is, if it is
	// a WildcardUri struct.
	IsWildcard() bool
}

// A SIP or SIPS URI, including all params and URI header params.
type SipUri struct {
	// True if and only if the URI is a SIPS URI.
	IsEncrypted bool

	// The user part of the URI: the 'joe' in sip:joe@bloggs.com
	// This is a pointer, so that URIs without a user part can have 'nil'.
	User *string

	// The password field of the URI. This is represented in the URI as joe:hunter2@bloggs.com.
	// Note that if a URI has a password field, it *must* have a user field as well.
	// This is a pointer, so that URIs without a password field can have 'nil'.
	// Note that RFC 3261 strongly recommends against the use of password fields in SIP URIs,
	// as they are fundamentally insecure.
	Password *string

	// The host part of the URI. This can be a domain, or a string representation of an IP address.
	Host string

	// The port part of the URI. This is optional, and so is represented here as a pointer type.
	Port *uint16

	// Any parameters associated with the URI.
	// These are used to provide information about requests that may be constructed from the URI.
	// (For more details, see RFC 3261 section 19.1.1).
	// These appear as a semicolon-separated list of key=value pairs following the host[:port] part.
	// Note that not all keys have an associated value, so the values of the map may be nil.
	UriParams map[string]*string

	// Any headers to be included on requests constructed from this URI.
	// These appear as a '&'-separated list at the end of the URI, introduced by '?'.
	// Although the values of the map are pointers, they will never be nil in practice as the parser
	// guarantees to not return nil values for header elements in SIP URIs.
	// You should not set the values of headers to nil.
	Headers map[string]*string
}

// IsWildcard() always returns 'false' for SIP URIs as they are not equal to the wildcard '*' URI.
// This method is required since SIP URIs are valid in Contact: headers.
func (uri *SipUri) IsWildcard() bool {
	return false
}

// Determine if the SIP URI is equal to the specified URI according to the rules laid down in RFC 3261 s. 19.1.4.
// TODO: The Equals method is not currently RFC-compliant; fix this!
func (uri *SipUri) Equals(otherUri Uri) bool {
	otherPtr, ok := otherUri.(*SipUri)
	if !ok {
		return false
	}

	other := *otherPtr
	result := uri.IsEncrypted == other.IsEncrypted &&
		strPtrEq(uri.User, other.User) &&
		strPtrEq(uri.Password, other.Password) &&
		uri.Host == other.Host &&
		uint16PtrEq(uri.Port, other.Port)

	if !result {
		return false
	}

	if !paramsEqual(uri.UriParams, other.UriParams) {
		return false
	}

	if !paramsEqual(uri.Headers, other.Headers) {
		return false
	}

	return true
}

// Generates the string representation of a SipUri struct.
func (uri *SipUri) String() string {
	var buffer bytes.Buffer

	// Compulsory protocol identifier.
	if uri.IsEncrypted {
		buffer.WriteString("sips")
		buffer.WriteString(":")
	} else {
		buffer.WriteString("sip")
		buffer.WriteString(":")
	}

	// Optional userinfo part.
	if uri.User != nil {
		buffer.WriteString(*uri.User)

		if uri.Password != nil {
			buffer.WriteString(":")
			buffer.WriteString(*uri.Password)
		}

		buffer.WriteString("@")
	}

	// Compulsory hostname.
	buffer.WriteString(uri.Host)

	// Optional port number.
	if uri.Port != nil {
		buffer.WriteString(":")
		buffer.WriteString(strconv.Itoa(int(*uri.Port)))
	}

	buffer.WriteString(ParamsToString(uri.UriParams, ';', ';'))
	buffer.WriteString(ParamsToString(uri.Headers, '?', '&'))

	return buffer.String()
}

// The special wildcard URI used in Contact: headers in REGISTER requests when expiring all registrations.
type WildcardUri struct{}

// Always returns 'true'.
func (uri *WildcardUri) IsWildcard() bool {
	return true
}

// Always returns '*' - the representation of a wildcard URI in a SIP message.
func (uri *WildcardUri) String() string {
	return "*"
}

// Determines if this wildcard URI equals the specified other URI.
// This is true if and only if the other URI is also a wildcard URI.
func (uri *WildcardUri) Equals(other Uri) bool {
	switch other.(type) {
	case *WildcardUri:
		return true
	default:
		return false
	}
}

// Encapsulates a header that gossip does not natively support.
// This allows header data that is not understood to be parsed by gossip and relayed to the parent application.
type GenericHeader struct {
	// The name of the header.
	headerName string

	// The contents of the header, including any parameters.
	// This is transparent data that is not natively understood by gossip.
	contents string
}

// Convert the header to a flat string representation.
func (header *GenericHeader) String() string {
	return header.headerName + ": " + header.contents
}

type ToHeader struct {
	// The display name from the header - this is a pointer type as it is optional.
	displayName *string

	uri Uri

	// Any parameters present in the header.
	params map[string]*string
}

func (to *ToHeader) String() string {
	var buffer bytes.Buffer
	buffer.WriteString("To: ")

	if to.displayName != nil {
		buffer.WriteString(fmt.Sprintf("\"%s\" ", *to.displayName))
	}

	buffer.WriteString(fmt.Sprintf("<%s>", to.uri))
	buffer.WriteString(ParamsToString(to.params, ';', ';'))

	return buffer.String()
}

type FromHeader struct {
	// The display name from the header - this is a pointer type as it is optional.
	displayName *string

	uri Uri

	// Any parameters present in the header.
	params map[string]*string
}

func (from *FromHeader) String() string {
	var buffer bytes.Buffer
	buffer.WriteString("From: ")

	if from.displayName != nil {
		buffer.WriteString(fmt.Sprintf("\"%s\" ", *from.displayName))
	}

	buffer.WriteString(fmt.Sprintf("<%s>", from.uri))
	buffer.WriteString(ParamsToString(from.params, ';', ';'))

	return buffer.String()
}

type ContactHeader struct {
	// The display name from the header - this is a pointer type as it is optional.
	displayName *string

	uri ContactUri

	// Any parameters present in the header.
	params map[string]*string
}

func (contact *ContactHeader) String() string {
	var buffer bytes.Buffer
	buffer.WriteString("Contact: ")

	if contact.displayName != nil {
		buffer.WriteString(fmt.Sprintf("\"%s\" ", *contact.displayName))
	}

	switch contact.uri.(type) {
	case *WildcardUri:
		// Treat the Wildcard URI separately as it must not be contained in < > angle brackets.
		buffer.WriteString("*")
	default:
		buffer.WriteString(fmt.Sprintf("<%s>", contact.uri.String()))
	}

	buffer.WriteString(ParamsToString(contact.params, ';', ';'))

	return buffer.String()
}

type CallId string

func (callId *CallId) String() string {
	return "Call-Id: " + (string)(*callId)
}

type CSeq struct {
	SeqNo      uint32
	MethodName Method
}

func (cseq *CSeq) String() string {
	return fmt.Sprintf("CSeq: %d %s", cseq.SeqNo, cseq.MethodName)
}

type MaxForwards uint32

func (maxForwards *MaxForwards) String() string {
	return fmt.Sprintf("Max-Forwards: %d", ((int)(*maxForwards)))
}

type ContentLength uint32

func (contentLength *ContentLength) String() string {
	return fmt.Sprintf("Content-Length: %d", ((int)(*contentLength)))
}

type ViaHeader []*ViaHop

// A single component in a Via header.
// Via headers are composed of several segments of the same structure, added by successive nodes in a routing chain.
type ViaHop struct {
	// E.g. 'SIP'.
	protocolName string

	// E.g. '2.0'.
	protocolVersion string
	transport       string
	host            string

	// The port for this via hop. This is stored as a pointer type, since it is an optional field.
	port *uint16

	params map[string]*string
}

func (entry *ViaHop) String() string {
	var buffer bytes.Buffer
	buffer.WriteString(fmt.Sprintf("%s/%s/%s %s",
		entry.protocolName, entry.protocolVersion,
		entry.transport,
		entry.host))
	if entry.port != nil {
		buffer.WriteString(fmt.Sprintf(":%d", *entry.port))
	}

	buffer.WriteString(ParamsToString(entry.params, ';', ';'))

	return buffer.String()
}

func (via ViaHeader) String() string {
	var buffer bytes.Buffer
	buffer.WriteString("Via: ")
	for idx, entry := range via {
		buffer.WriteString(entry.String())
		if idx != len(via)-1 {
			buffer.WriteString(", ")
		}
	}

	return buffer.String()
}

type RequireHeader struct {
	options []string
}

func (header *RequireHeader) String() string {
	return fmt.Sprintf("Require: %s",
		strings.Join(header.options, ", "))
}

type SupportedHeader struct {
	options []string
}

func (header *SupportedHeader) String() string {
	return fmt.Sprintf("Supported: %s",
		strings.Join(header.options, ", "))
}

type ProxyRequireHeader struct {
	options []string
}

func (header *ProxyRequireHeader) String() string {
	return fmt.Sprintf("Proxy-Require: %s",
		strings.Join(header.options, ", "))
}

// 'Unsupported:' is a SIP header type - this doesn't indicate that the
// header itself is not supported by gossip!
type UnsupportedHeader struct {
	options []string
}

func (header *UnsupportedHeader) String() string {
	return fmt.Sprintf("Unsupported: %s",
		strings.Join(header.options, ", "))
}

// Utility method for converting a map of parameters to a flat string representation.
// Takes the map of parameters, and start and end characters (e.g. '?' and '&').
// It is assumed that key/value pairs are always represented as "key=value".
// Note that this method does not escape special characters - that should be done before calling this method.
func ParamsToString(params map[string]*string, start uint8, sep uint8) string {
	var buffer bytes.Buffer
	first := true
	for key, value := range params {
		if first {
			buffer.WriteString(fmt.Sprintf("%c", start))
			first = false
		} else {
			buffer.WriteString(fmt.Sprintf("%c", sep))
		}
		if value == nil {
			buffer.WriteString(fmt.Sprintf("%s", key))
		} else if strings.ContainsAny(*value, ABNF_WS) {
			buffer.WriteString(fmt.Sprintf("%s=\"%s\"", key, *value))
		} else {
			buffer.WriteString(fmt.Sprintf("%s=%s", key, *value))
		}
	}

	return buffer.String()
}

// Check if two maps of parameters are equal in the sense of having the same keys with the same values.
// This does not rely on any ordering of the keys of the map in memory.
func paramsEqual(a map[string]*string, b map[string]*string) bool {
	if len(a) != len(b) {
		return false
	}

	for key, a_val := range a {
		b_val, ok := b[key]
		if !ok {
			return false
		}
		if !strPtrEq(a_val, b_val) {
			return false
		}
	}

	return true
}
