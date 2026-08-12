package widget

import (
	"fmt"
	"strconv"
	"strings"
	"unicode/utf8"

	celltext "github.com/mayahiro/nagi-go/text"
)

const (
	// DefaultJSONDocumentMaxNodes is the default maximum JSON value occurrence count
	DefaultJSONDocumentMaxNodes uint64 = 100_000
	// DefaultJSONDocumentMaxDepth is the default maximum one-based JSON value depth
	DefaultJSONDocumentMaxDepth uint32 = 128
	// MaxJSONDocumentDepth is the hard maximum depth supported by the inspector Node backend
	MaxJSONDocumentDepth uint32 = 256
	// DefaultJSONDocumentMaxStringBytes is the default decoded string and key byte limit
	DefaultJSONDocumentMaxStringBytes uint64 = 16 * 1024 * 1024
	// DefaultJSONDocumentMaxSerializedBytes is the default compact serialization byte limit
	DefaultJSONDocumentMaxSerializedBytes uint64 = 32 * 1024 * 1024
)

// JSONKind is one closed JSON value category
type JSONKind uint8

const (
	// JSONNullKind is JSON null
	JSONNullKind JSONKind = iota
	// JSONBooleanKind is a JSON Boolean
	JSONBooleanKind
	// JSONNumberKind is a JSON number token
	JSONNumberKind
	// JSONStringKind is a JSON string
	JSONStringKind
	// JSONArrayKind is an ordered JSON array
	JSONArrayKind
	// JSONObjectKind is an ordered JSON object
	JSONObjectKind
)

// String returns the stable language-independent kind identifier
func (k JSONKind) String() string {
	switch k {
	case JSONNullKind:
		return "null"
	case JSONBooleanKind:
		return "boolean"
	case JSONNumberKind:
		return "number"
	case JSONStringKind:
		return "string"
	case JSONArrayKind:
		return "array"
	case JSONObjectKind:
		return "object"
	default:
		return "unknown"
	}
}

// JSONNumber is one validated JSON number token that preserves its spelling
//
// The zero value represents the valid token 0
type JSONNumber struct {
	value string
}

// InvalidJSONNumberError reports a token outside the JSON number grammar
type InvalidJSONNumberError struct {
	// Offset is the first invalid or missing byte offset
	Offset int
	// ByteLength is the rejected token byte length
	ByteLength int
}

// Error returns a diagnostic that does not retain the rejected token
func (e *InvalidJSONNumberError) Error() string {
	return fmt.Sprintf("invalid JSON number at byte %d of %d", e.Offset, e.ByteLength)
}

// NewJSONNumber validates and owns one JSON number token
func NewJSONNumber(value string) (JSONNumber, error) {
	if offset, valid := validateJSONNumber(value); !valid {
		return JSONNumber{}, &InvalidJSONNumberError{Offset: offset, ByteLength: len(value)}
	}
	return JSONNumber{value: value}, nil
}

// String returns the validated token without numeric conversion
func (n JSONNumber) String() string {
	if n.value == "" {
		return "0"
	}
	return n.value
}

// JSONMember is one ordered JSON object member
type JSONMember struct {
	key   string
	value JSONValue
}

// NewJSONMember returns one key-value member with a valid UTF-8 key
func NewJSONMember(key string, value JSONValue) JSONMember {
	return JSONMember{key: celltext.NormalizeUTF8(key), value: value}
}

// Key returns the normalized member key
func (m JSONMember) Key() string {
	return m.key
}

// Value returns the immutable member value
func (m JSONMember) Value() JSONValue {
	return m.value
}

// DuplicateJSONKeyError reports one repeated normalized object key
type DuplicateJSONKeyError struct {
	// Key is the duplicated normalized key
	Key string
}

// Error returns the duplicate-key diagnostic
func (e *DuplicateJSONKeyError) Error() string {
	return fmt.Sprintf("duplicate JSON object key %s", e.Key)
}

// JSONValue is one immutable typed JSON value
//
// The zero value is JSON null
type JSONValue struct {
	inner *jsonValueData
}

type jsonValueData struct {
	kind    JSONKind
	boolean bool
	number  JSONNumber
	text    string
	array   []JSONValue
	object  []JSONMember
}

// NewJSONNull returns JSON null
func NewJSONNull() JSONValue {
	return JSONValue{}
}

// NewJSONBoolean returns a JSON Boolean
func NewJSONBoolean(value bool) JSONValue {
	return JSONValue{inner: &jsonValueData{kind: JSONBooleanKind, boolean: value}}
}

// NewJSONNumberValue returns a JSON number from a validated token
func NewJSONNumberValue(value JSONNumber) JSONValue {
	return JSONValue{inner: &jsonValueData{kind: JSONNumberKind, number: value}}
}

// NewJSONString returns a JSON string after invalid UTF-8 replacement
func NewJSONString(value string) JSONValue {
	return JSONValue{inner: &jsonValueData{kind: JSONStringKind, text: celltext.NormalizeUTF8(value)}}
}

// NewJSONArray returns an immutable ordered JSON array
func NewJSONArray(values []JSONValue) JSONValue {
	return JSONValue{inner: &jsonValueData{
		kind: JSONArrayKind, array: append([]JSONValue(nil), values...),
	}}
}

// NewJSONObject returns an immutable ordered JSON object and rejects duplicate normalized keys
func NewJSONObject(members []JSONMember) (JSONValue, error) {
	owned := append([]JSONMember(nil), members...)
	seen := make(map[string]struct{}, len(owned))
	for _, member := range owned {
		if _, exists := seen[member.key]; exists {
			return JSONValue{}, &DuplicateJSONKeyError{Key: member.key}
		}
		seen[member.key] = struct{}{}
	}
	return JSONValue{inner: &jsonValueData{kind: JSONObjectKind, object: owned}}, nil
}

// Kind returns the closed value category
func (v JSONValue) Kind() JSONKind {
	if v.inner == nil {
		return JSONNullKind
	}
	return v.inner.kind
}

// Boolean returns the Boolean value when this is a Boolean
func (v JSONValue) Boolean() (bool, bool) {
	if v.Kind() != JSONBooleanKind {
		return false, false
	}
	return v.inner.boolean, true
}

// Number returns the validated number token when this is a Number
func (v JSONValue) Number() (JSONNumber, bool) {
	if v.Kind() != JSONNumberKind {
		return JSONNumber{}, false
	}
	return v.inner.number, true
}

// Text returns the decoded normalized string when this is a String
func (v JSONValue) Text() (string, bool) {
	if v.Kind() != JSONStringKind {
		return "", false
	}
	return v.inner.text, true
}

// Array returns an independent ordered value slice when this is an Array
func (v JSONValue) Array() ([]JSONValue, bool) {
	if v.Kind() != JSONArrayKind {
		return nil, false
	}
	return append([]JSONValue(nil), v.inner.array...), true
}

// Object returns an independent ordered member slice when this is an Object
func (v JSONValue) Object() ([]JSONMember, bool) {
	if v.Kind() != JSONObjectKind {
		return nil, false
	}
	return append([]JSONMember(nil), v.inner.object...), true
}

func (v JSONValue) arrayValues() []JSONValue {
	if v.Kind() != JSONArrayKind {
		return nil
	}
	return v.inner.array
}

func (v JSONValue) objectMembers() []JSONMember {
	if v.Kind() != JSONObjectKind {
		return nil
	}
	return v.inner.object
}

// JSONPointer is one validated RFC 6901 pointer used as a document identity
//
// The zero value is the root pointer
type JSONPointer struct {
	value string
}

// InvalidJSONPointerError reports an invalid RFC 6901 pointer
type InvalidJSONPointerError struct {
	// Offset is the first invalid or missing byte offset
	Offset int
}

// Error returns the pointer diagnostic
func (e *InvalidJSONPointerError) Error() string {
	return fmt.Sprintf("invalid JSON Pointer at byte %d", e.Offset)
}

// RootJSONPointer returns the root pointer
func RootJSONPointer() JSONPointer {
	return JSONPointer{}
}

// NewJSONPointer normalizes, validates, and owns one RFC 6901 pointer
func NewJSONPointer(value string) (JSONPointer, error) {
	value = celltext.NormalizeUTF8(value)
	if offset, valid := validateJSONPointer(value); !valid {
		return JSONPointer{}, &InvalidJSONPointerError{Offset: offset}
	}
	return JSONPointer{value: value}, nil
}

// String returns the encoded pointer
func (p JSONPointer) String() string {
	return p.value
}

// JSONDocumentLimits bounds eager work for one JSONDocument
//
// The zero value uses every documented default
type JSONDocumentLimits struct {
	maxNodes           uint64
	maxDepth           uint32
	maxStringBytes     uint64
	maxSerializedBytes uint64
}

// DefaultJSONDocumentLimits returns every documented default resource limit
func DefaultJSONDocumentLimits() JSONDocumentLimits {
	return JSONDocumentLimits{
		maxNodes:           DefaultJSONDocumentMaxNodes,
		maxDepth:           DefaultJSONDocumentMaxDepth,
		maxStringBytes:     DefaultJSONDocumentMaxStringBytes,
		maxSerializedBytes: DefaultJSONDocumentMaxSerializedBytes,
	}
}

// MaxNodes returns the maximum JSON value occurrence count
func (l JSONDocumentLimits) MaxNodes() uint64 {
	return l.normalized().maxNodes
}

// WithMaxNodes returns these limits with a maximum JSON value occurrence count
func (l JSONDocumentLimits) WithMaxNodes(value uint64) JSONDocumentLimits {
	l = l.normalized()
	if value == 0 {
		value = DefaultJSONDocumentMaxNodes
	}
	l.maxNodes = value
	return l
}

// MaxDepth returns the capped maximum one-based JSON value depth
func (l JSONDocumentLimits) MaxDepth() uint32 {
	return l.normalized().maxDepth
}

// WithMaxDepth returns these limits with a capped maximum one-based depth
func (l JSONDocumentLimits) WithMaxDepth(value uint32) JSONDocumentLimits {
	l = l.normalized()
	if value == 0 {
		value = DefaultJSONDocumentMaxDepth
	}
	l.maxDepth = min(value, MaxJSONDocumentDepth)
	return l
}

// MaxStringBytes returns the maximum decoded bytes across strings and object keys
func (l JSONDocumentLimits) MaxStringBytes() uint64 {
	return l.normalized().maxStringBytes
}

// WithMaxStringBytes returns these limits with a maximum decoded string byte count
func (l JSONDocumentLimits) WithMaxStringBytes(value uint64) JSONDocumentLimits {
	l = l.normalized()
	if value == 0 {
		value = DefaultJSONDocumentMaxStringBytes
	}
	l.maxStringBytes = value
	return l
}

// MaxSerializedBytes returns the maximum deterministic compact serialization byte count
func (l JSONDocumentLimits) MaxSerializedBytes() uint64 {
	return l.normalized().maxSerializedBytes
}

// WithMaxSerializedBytes returns these limits with a maximum serialized byte count
func (l JSONDocumentLimits) WithMaxSerializedBytes(value uint64) JSONDocumentLimits {
	l = l.normalized()
	if value == 0 {
		value = DefaultJSONDocumentMaxSerializedBytes
	}
	l.maxSerializedBytes = value
	return l
}

func (l JSONDocumentLimits) normalized() JSONDocumentLimits {
	defaults := DefaultJSONDocumentLimits()
	if l.maxNodes == 0 {
		l.maxNodes = defaults.maxNodes
	}
	if l.maxDepth == 0 {
		l.maxDepth = defaults.maxDepth
	}
	l.maxDepth = min(l.maxDepth, MaxJSONDocumentDepth)
	if l.maxStringBytes == 0 {
		l.maxStringBytes = defaults.maxStringBytes
	}
	if l.maxSerializedBytes == 0 {
		l.maxSerializedBytes = defaults.maxSerializedBytes
	}
	return l
}

// JSONDocumentErrorKind is one stable JSON document resource failure category
type JSONDocumentErrorKind uint8

const (
	// JSONDocumentNodeLimit means the value occurrence limit was exceeded
	JSONDocumentNodeLimit JSONDocumentErrorKind = iota
	// JSONDocumentDepthLimit means the one-based nesting depth limit was exceeded
	JSONDocumentDepthLimit
	// JSONDocumentStringByteLimit means the decoded string and key byte limit was exceeded
	JSONDocumentStringByteLimit
	// JSONDocumentSerializedByteLimit means the compact serialization byte limit was exceeded
	JSONDocumentSerializedByteLimit
)

// String returns the stable language-independent error identifier
func (k JSONDocumentErrorKind) String() string {
	switch k {
	case JSONDocumentNodeLimit:
		return "node-limit"
	case JSONDocumentDepthLimit:
		return "depth-limit"
	case JSONDocumentStringByteLimit:
		return "string-byte-limit"
	case JSONDocumentSerializedByteLimit:
		return "serialized-byte-limit"
	default:
		return "unknown"
	}
}

// JSONDocumentError is one structured resource failure
type JSONDocumentError struct {
	// Kind is the stable resource category
	Kind JSONDocumentErrorKind
	// Limit is the configured limit
	Limit uint64
	// Observed is the first observed value beyond the limit
	Observed uint64
}

// Error returns the resource diagnostic
func (e *JSONDocumentError) Error() string {
	return fmt.Sprintf(
		"JSON document %s: limit %d, observed %d", e.Kind.String(), e.Limit, e.Observed,
	)
}

// JSONDocument is an immutable indexed JSON document with deterministic compact serialization
//
// The zero value is a document containing JSON null
type JSONDocument struct {
	inner *jsonDocumentData
}

type jsonDocumentData struct {
	serialized string
	nodes      []jsonDocumentNode
	positions  map[JSONPointer]int
}

type jsonDocumentNode struct {
	path            JSONPointer
	kind            JSONKind
	depth           uint32
	parent          int
	hasParent       bool
	childCount      int
	subtreeEnd      int
	serializedStart int
	serializedEnd   int
	label           jsonNodeLabel
	decodedString   string
}

type jsonNodeLabel struct {
	kind  uint8
	key   string
	index int
}

const (
	jsonNodeRootLabel uint8 = iota
	jsonNodeObjectKeyLabel
	jsonNodeArrayIndexLabel
)

var zeroJSONDocumentData = jsonDocumentData{
	serialized: "null",
	nodes: []jsonDocumentNode{{
		kind: JSONNullKind, parent: -1, subtreeEnd: 1,
		serializedEnd: 4,
	}},
	positions: map[JSONPointer]int{{}: 0},
}

// NewJSONDocument builds a document using bounded default limits
func NewJSONDocument(root JSONValue) (JSONDocument, error) {
	return NewJSONDocumentWithLimits(root, DefaultJSONDocumentLimits())
}

// NewJSONDocumentWithLimits builds a document after validating every configured resource limit
func NewJSONDocumentWithLimits(root JSONValue, limits JSONDocumentLimits) (JSONDocument, error) {
	limits = limits.normalized()
	validation, err := validateJSONDocument(root, limits)
	if err != nil {
		return JSONDocument{}, err
	}
	var builder strings.Builder
	if validation.serializedBytes <= uint64(int(^uint(0)>>1)) {
		builder.Grow(int(validation.serializedBytes))
	}
	capacity := 0
	if validation.nodes <= uint64(int(^uint(0)>>1)) {
		capacity = int(validation.nodes)
	}
	nodes := make([]jsonDocumentNode, 0, capacity)
	buildJSONDocumentNode(root, jsonNodeLabel{kind: jsonNodeRootLabel}, -1, 0, JSONPointer{}, &builder, &nodes)
	positions := make(map[JSONPointer]int, len(nodes))
	for index := range nodes {
		positions[nodes[index].path] = index
	}
	return JSONDocument{inner: &jsonDocumentData{
		serialized: builder.String(), nodes: nodes, positions: positions,
	}}, nil
}

// Serialized returns the complete deterministic compact JSON serialization
func (d JSONDocument) Serialized() string {
	return d.data().serialized
}

// Len returns the number of indexed JSON values
func (d JSONDocument) Len() int {
	return len(d.data().nodes)
}

// Empty reports whether the document has no indexed value
//
// A JSONDocument always contains one root value, including its zero value
func (d JSONDocument) Empty() bool {
	return d.Len() == 0
}

// Root returns the root JSON value
func (d JSONDocument) Root() JSONNode {
	return JSONNode{document: d.data(), index: 0}
}

// Get looks up one value by complete JSON Pointer
func (d JSONDocument) Get(pointer JSONPointer) (JSONNode, bool) {
	index, ok := d.indexOf(pointer)
	if !ok {
		return JSONNode{}, false
	}
	return JSONNode{document: d.data(), index: index}, true
}

// Nodes returns an allocation-free preorder iterator
func (d JSONDocument) Nodes() JSONNodes {
	return JSONNodes{document: d.data()}
}

func (d JSONDocument) data() *jsonDocumentData {
	if d.inner == nil {
		return &zeroJSONDocumentData
	}
	return d.inner
}

func (d JSONDocument) indexOf(pointer JSONPointer) (int, bool) {
	index, ok := d.data().positions[pointer]
	return index, ok
}

func (d JSONDocument) nodeAt(index int) JSONNode {
	return JSONNode{document: d.data(), index: index}
}

func (d JSONDocument) record(index int) *jsonDocumentNode {
	return &d.data().nodes[index]
}

// JSONNode is a borrowed read-only view of one indexed JSON value
type JSONNode struct {
	document *jsonDocumentData
	index    int
}

// Kind returns the value category
func (n JSONNode) Kind() JSONKind {
	return n.record().kind
}

// Path returns the stable complete JSON Pointer
func (n JSONNode) Path() JSONPointer {
	return n.record().path
}

// Depth returns the zero-based value depth
func (n JSONNode) Depth() uint32 {
	return n.record().depth
}

// ChildCount returns the direct child count
func (n JSONNode) ChildCount() int {
	return n.record().childCount
}

// Key returns this member's object key when present
func (n JSONNode) Key() (string, bool) {
	record := n.record()
	return record.label.key, record.label.kind == jsonNodeObjectKeyLabel
}

// ArrayIndex returns this member's array index when present
func (n JSONNode) ArrayIndex() (int, bool) {
	record := n.record()
	return record.label.index, record.label.kind == jsonNodeArrayIndexLabel
}

// Serialized returns the complete compact serialization of this value
func (n JSONNode) Serialized() string {
	record := n.record()
	return n.data().serialized[record.serializedStart:record.serializedEnd]
}

func (n JSONNode) data() *jsonDocumentData {
	if n.document == nil {
		return &zeroJSONDocumentData
	}
	return n.document
}

func (n JSONNode) record() *jsonDocumentNode {
	data := n.data()
	if n.index < 0 || n.index >= len(data.nodes) {
		return &zeroJSONDocumentData.nodes[0]
	}
	return &data.nodes[n.index]
}

// JSONNodes traverses one immutable document in deterministic preorder
type JSONNodes struct {
	document *jsonDocumentData
	next     int
}

// Next returns the next value and whether one was available
func (n *JSONNodes) Next() (JSONNode, bool) {
	if n == nil {
		return JSONNode{}, false
	}
	document := n.document
	if document == nil {
		document = &zeroJSONDocumentData
	}
	if n.next >= len(document.nodes) {
		return JSONNode{}, false
	}
	node := JSONNode{document: document, index: n.next}
	n.next++
	return node, true
}

// Remaining returns the exact number of values not yet returned
func (n *JSONNodes) Remaining() int {
	if n == nil {
		return 0
	}
	document := n.document
	if document == nil {
		document = &zeroJSONDocumentData
	}
	return max(len(document.nodes)-n.next, 0)
}

func validateJSONNumber(value string) (int, bool) {
	index := 0
	if index < len(value) && value[index] == '-' {
		index++
	}
	if index >= len(value) {
		return index, false
	}
	switch value[index] {
	case '0':
		index++
		if index < len(value) && isASCIIDigit(value[index]) {
			return index, false
		}
	default:
		if value[index] < '1' || value[index] > '9' {
			return index, false
		}
		index++
		for index < len(value) && isASCIIDigit(value[index]) {
			index++
		}
	}
	if index < len(value) && value[index] == '.' {
		index++
		if index >= len(value) || !isASCIIDigit(value[index]) {
			return index, false
		}
		for index < len(value) && isASCIIDigit(value[index]) {
			index++
		}
	}
	if index < len(value) && (value[index] == 'e' || value[index] == 'E') {
		index++
		if index < len(value) && (value[index] == '+' || value[index] == '-') {
			index++
		}
		if index >= len(value) || !isASCIIDigit(value[index]) {
			return index, false
		}
		for index < len(value) && isASCIIDigit(value[index]) {
			index++
		}
	}
	return index, index == len(value)
}

func isASCIIDigit(value byte) bool {
	return value >= '0' && value <= '9'
}

func validateJSONPointer(value string) (int, bool) {
	if value == "" {
		return 0, true
	}
	if value[0] != '/' {
		return 0, false
	}
	for index := 0; index < len(value); index++ {
		if value[index] != '~' {
			continue
		}
		if index+1 >= len(value) || (value[index+1] != '0' && value[index+1] != '1') {
			return index, false
		}
		index++
	}
	return len(value), true
}

type jsonDocumentValidation struct {
	nodes           uint64
	serializedBytes uint64
}

func validateJSONDocument(root JSONValue, limits JSONDocumentLimits) (jsonDocumentValidation, error) {
	counters := jsonDocumentCounters{}
	if err := validateJSONDocumentValue(root, 1, limits, &counters); err != nil {
		return jsonDocumentValidation{}, err
	}
	return jsonDocumentValidation{
		nodes: counters.nodes, serializedBytes: counters.serializedBytes,
	}, nil
}

type jsonDocumentCounters struct {
	nodes           uint64
	stringBytes     uint64
	serializedBytes uint64
}

func validateJSONDocumentValue(
	value JSONValue,
	depth uint32,
	limits JSONDocumentLimits,
	counters *jsonDocumentCounters,
) error {
	counters.nodes = saturatingAdd(counters.nodes, 1)
	if err := checkJSONLimit(JSONDocumentNodeLimit, limits.maxNodes, counters.nodes); err != nil {
		return err
	}
	if err := checkJSONLimit(
		JSONDocumentDepthLimit, uint64(limits.maxDepth), uint64(depth),
	); err != nil {
		return err
	}
	switch value.Kind() {
	case JSONNullKind:
		counters.serializedBytes = saturatingAdd(counters.serializedBytes, 4)
	case JSONBooleanKind:
		boolean, _ := value.Boolean()
		if boolean {
			counters.serializedBytes = saturatingAdd(counters.serializedBytes, 4)
		} else {
			counters.serializedBytes = saturatingAdd(counters.serializedBytes, 5)
		}
	case JSONNumberKind:
		number, _ := value.Number()
		counters.serializedBytes = saturatingAdd(counters.serializedBytes, uint64(len(number.String())))
	case JSONStringKind:
		text, _ := value.Text()
		counters.stringBytes = saturatingAdd(counters.stringBytes, uint64(len(text)))
		counters.serializedBytes = saturatingAdd(counters.serializedBytes, jsonStringSerializedBytes(text))
	case JSONArrayKind:
		counters.serializedBytes = saturatingAdd(
			counters.serializedBytes, jsonCollectionPunctuation(len(value.arrayValues())),
		)
	case JSONObjectKind:
		members := value.objectMembers()
		counters.serializedBytes = saturatingAdd(
			counters.serializedBytes, jsonCollectionPunctuation(len(members)),
		)
		for _, member := range members {
			counters.stringBytes = saturatingAdd(counters.stringBytes, uint64(len(member.key)))
			counters.serializedBytes = saturatingAdd(
				counters.serializedBytes, saturatingAdd(jsonStringSerializedBytes(member.key), 1),
			)
		}
	default:
		counters.serializedBytes = saturatingAdd(counters.serializedBytes, 4)
	}
	if err := checkJSONLimit(
		JSONDocumentStringByteLimit, limits.maxStringBytes, counters.stringBytes,
	); err != nil {
		return err
	}
	if err := checkJSONLimit(
		JSONDocumentSerializedByteLimit, limits.maxSerializedBytes, counters.serializedBytes,
	); err != nil {
		return err
	}
	switch value.Kind() {
	case JSONArrayKind:
		for _, child := range value.arrayValues() {
			if err := validateJSONDocumentValue(child, depth+1, limits, counters); err != nil {
				return err
			}
		}
	case JSONObjectKind:
		for _, member := range value.objectMembers() {
			if err := validateJSONDocumentValue(member.value, depth+1, limits, counters); err != nil {
				return err
			}
		}
	}
	return nil
}

func checkJSONLimit(kind JSONDocumentErrorKind, limit, observed uint64) error {
	if observed <= limit {
		return nil
	}
	return &JSONDocumentError{Kind: kind, Limit: limit, Observed: observed}
}

func saturatingAdd(left, right uint64) uint64 {
	if ^uint64(0)-left < right {
		return ^uint64(0)
	}
	return left + right
}

func jsonCollectionPunctuation(count int) uint64 {
	if count == 0 {
		return 2
	}
	return saturatingAdd(2, uint64(count-1))
}

func jsonStringSerializedBytes(value string) uint64 {
	bytes := uint64(2)
	for _, character := range value {
		switch character {
		case '"', '\\', '\b', '\f', '\n', '\r', '\t':
			bytes = saturatingAdd(bytes, 2)
		default:
			if character < 0x20 {
				bytes = saturatingAdd(bytes, 6)
			} else {
				bytes = saturatingAdd(bytes, uint64(utf8.RuneLen(character)))
			}
		}
	}
	return bytes
}

func buildJSONDocumentNode(
	value JSONValue,
	label jsonNodeLabel,
	parent int,
	depth uint32,
	path JSONPointer,
	output *strings.Builder,
	nodes *[]jsonDocumentNode,
) {
	index := len(*nodes)
	start := output.Len()
	record := jsonDocumentNode{
		path: path, kind: value.Kind(), depth: depth, parent: parent,
		hasParent: parent >= 0, subtreeEnd: index + 1, serializedStart: start,
		label: label,
	}
	switch value.Kind() {
	case JSONStringKind:
		record.decodedString, _ = value.Text()
	case JSONArrayKind:
		record.childCount = len(value.arrayValues())
	case JSONObjectKind:
		record.childCount = len(value.objectMembers())
	}
	*nodes = append(*nodes, record)

	switch value.Kind() {
	case JSONNullKind:
		output.WriteString("null")
	case JSONBooleanKind:
		boolean, _ := value.Boolean()
		if boolean {
			output.WriteString("true")
		} else {
			output.WriteString("false")
		}
	case JSONNumberKind:
		number, _ := value.Number()
		output.WriteString(number.String())
	case JSONStringKind:
		text, _ := value.Text()
		writeJSONString(output, text)
	case JSONArrayKind:
		output.WriteByte('[')
		for childIndex, child := range value.arrayValues() {
			if childIndex > 0 {
				output.WriteByte(',')
			}
			buildJSONDocumentNode(
				child,
				jsonNodeLabel{kind: jsonNodeArrayIndexLabel, index: childIndex},
				index,
				depth+1,
				arrayJSONPointer(path, childIndex),
				output,
				nodes,
			)
		}
		output.WriteByte(']')
	case JSONObjectKind:
		output.WriteByte('{')
		for memberIndex, member := range value.objectMembers() {
			if memberIndex > 0 {
				output.WriteByte(',')
			}
			writeJSONString(output, member.key)
			output.WriteByte(':')
			buildJSONDocumentNode(
				member.value,
				jsonNodeLabel{kind: jsonNodeObjectKeyLabel, key: member.key},
				index,
				depth+1,
				objectJSONPointer(path, member.key),
				output,
				nodes,
			)
		}
		output.WriteByte('}')
	default:
		output.WriteString("null")
	}
	(*nodes)[index].serializedEnd = output.Len()
	(*nodes)[index].subtreeEnd = len(*nodes)
}

func arrayJSONPointer(parent JSONPointer, index int) JSONPointer {
	var digits [24]byte
	encoded := strconv.AppendInt(digits[:0], int64(index), 10)
	var builder strings.Builder
	builder.Grow(len(parent.value) + len(encoded) + 1)
	builder.WriteString(parent.value)
	builder.WriteByte('/')
	builder.Write(encoded)
	return JSONPointer{value: builder.String()}
}

func objectJSONPointer(parent JSONPointer, key string) JSONPointer {
	var builder strings.Builder
	builder.Grow(len(parent.value) + len(key) + 1)
	builder.WriteString(parent.value)
	builder.WriteByte('/')
	for _, character := range key {
		switch character {
		case '~':
			builder.WriteString("~0")
		case '/':
			builder.WriteString("~1")
		default:
			builder.WriteRune(character)
		}
	}
	return JSONPointer{value: builder.String()}
}

func writeJSONString(output *strings.Builder, value string) {
	output.WriteByte('"')
	writeJSONStringContent(output, value)
	output.WriteByte('"')
}

func writeJSONStringContent(output *strings.Builder, value string) {
	const hexadecimal = "0123456789ABCDEF"
	for _, character := range value {
		switch character {
		case '"':
			output.WriteString("\\\"")
		case '\\':
			output.WriteString("\\\\")
		case '\b':
			output.WriteString("\\b")
		case '\f':
			output.WriteString("\\f")
		case '\n':
			output.WriteString("\\n")
		case '\r':
			output.WriteString("\\r")
		case '\t':
			output.WriteString("\\t")
		default:
			if character < 0x20 {
				output.WriteString("\\u00")
				output.WriteByte(hexadecimal[(character>>4)&0x0F])
				output.WriteByte(hexadecimal[character&0x0F])
			} else {
				output.WriteRune(character)
			}
		}
	}
}
