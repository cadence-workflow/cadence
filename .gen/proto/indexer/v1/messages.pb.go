// The MIT License (MIT)

// Copyright (c) 2017-2020 Uber Technologies Inc.

// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in all
// copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
// SOFTWARE.

// Code generated by protoc-gen-gogo. DO NOT EDIT.
// source: uber/cadence/indexer/v1/messages.proto

package indexerv1

import (
	fmt "fmt"
	io "io"
	math "math"
	math_bits "math/bits"

	proto "github.com/gogo/protobuf/proto"

	v1 "github.com/uber/cadence/.gen/proto/api/v1"
)

// Reference imports to suppress errors if they are not otherwise used.
var _ = proto.Marshal
var _ = fmt.Errorf
var _ = math.Inf

// This is a compile-time assertion to ensure that this generated file
// is compatible with the proto package it is being compiled against.
// A compilation error at this line likely means your copy of the
// proto package needs to be updated.
const _ = proto.GoGoProtoPackageIsVersion3 // please upgrade the proto package

type MessageType int32

const (
	MessageType_MESSAGE_TYPE_INVALID MessageType = 0
	MessageType_MESSAGE_TYPE_INDEX   MessageType = 1
	MessageType_MESSAGE_TYPE_DELETE  MessageType = 2
)

var MessageType_name = map[int32]string{
	0: "MESSAGE_TYPE_INVALID",
	1: "MESSAGE_TYPE_INDEX",
	2: "MESSAGE_TYPE_DELETE",
}

var MessageType_value = map[string]int32{
	"MESSAGE_TYPE_INVALID": 0,
	"MESSAGE_TYPE_INDEX":   1,
	"MESSAGE_TYPE_DELETE":  2,
}

func (x MessageType) String() string {
	return proto.EnumName(MessageType_name, int32(x))
}

func (MessageType) EnumDescriptor() ([]byte, []int) {
	return fileDescriptor_60256a432328b016, []int{0}
}

type Message struct {
	MessageType          MessageType           `protobuf:"varint,1,opt,name=message_type,json=messageType,proto3,enum=server.uber.cadence.indexer.v1.MessageType" json:"message_type,omitempty"`
	DomainId             string                `protobuf:"bytes,2,opt,name=domain_id,json=domainId,proto3" json:"domain_id,omitempty"`
	WorkflowExecution    *v1.WorkflowExecution `protobuf:"bytes,3,opt,name=workflow_execution,json=workflowExecution,proto3" json:"workflow_execution,omitempty"`
	Version              int64                 `protobuf:"varint,4,opt,name=version,proto3" json:"version,omitempty"`
	Fields               map[string]*Field     `protobuf:"bytes,5,rep,name=fields,proto3" json:"fields,omitempty" protobuf_key:"bytes,1,opt,name=key,proto3" protobuf_val:"bytes,2,opt,name=value,proto3"`
	XXX_NoUnkeyedLiteral struct{}              `json:"-"`
	XXX_unrecognized     []byte                `json:"-"`
	XXX_sizecache        int32                 `json:"-"`
}

func (m *Message) Reset()         { *m = Message{} }
func (m *Message) String() string { return proto.CompactTextString(m) }
func (*Message) ProtoMessage()    {}
func (*Message) Descriptor() ([]byte, []int) {
	return fileDescriptor_60256a432328b016, []int{0}
}
func (m *Message) XXX_Unmarshal(b []byte) error {
	return m.Unmarshal(b)
}
func (m *Message) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	if deterministic {
		return xxx_messageInfo_Message.Marshal(b, m, deterministic)
	} else {
		b = b[:cap(b)]
		n, err := m.MarshalToSizedBuffer(b)
		if err != nil {
			return nil, err
		}
		return b[:n], nil
	}
}
func (m *Message) XXX_Merge(src proto.Message) {
	xxx_messageInfo_Message.Merge(m, src)
}
func (m *Message) XXX_Size() int {
	return m.Size()
}
func (m *Message) XXX_DiscardUnknown() {
	xxx_messageInfo_Message.DiscardUnknown(m)
}

var xxx_messageInfo_Message proto.InternalMessageInfo

func (m *Message) GetMessageType() MessageType {
	if m != nil {
		return m.MessageType
	}
	return MessageType_MESSAGE_TYPE_INVALID
}

func (m *Message) GetDomainId() string {
	if m != nil {
		return m.DomainId
	}
	return ""
}

func (m *Message) GetWorkflowExecution() *v1.WorkflowExecution {
	if m != nil {
		return m.WorkflowExecution
	}
	return nil
}

func (m *Message) GetVersion() int64 {
	if m != nil {
		return m.Version
	}
	return 0
}

func (m *Message) GetFields() map[string]*Field {
	if m != nil {
		return m.Fields
	}
	return nil
}

type Field struct {
	// Types that are valid to be assigned to Data:
	//	*Field_StringData
	//	*Field_IntData
	//	*Field_BoolData
	//	*Field_BinaryData
	Data                 isField_Data `protobuf_oneof:"data"`
	XXX_NoUnkeyedLiteral struct{}     `json:"-"`
	XXX_unrecognized     []byte       `json:"-"`
	XXX_sizecache        int32        `json:"-"`
}

func (m *Field) Reset()         { *m = Field{} }
func (m *Field) String() string { return proto.CompactTextString(m) }
func (*Field) ProtoMessage()    {}
func (*Field) Descriptor() ([]byte, []int) {
	return fileDescriptor_60256a432328b016, []int{1}
}
func (m *Field) XXX_Unmarshal(b []byte) error {
	return m.Unmarshal(b)
}
func (m *Field) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	if deterministic {
		return xxx_messageInfo_Field.Marshal(b, m, deterministic)
	} else {
		b = b[:cap(b)]
		n, err := m.MarshalToSizedBuffer(b)
		if err != nil {
			return nil, err
		}
		return b[:n], nil
	}
}
func (m *Field) XXX_Merge(src proto.Message) {
	xxx_messageInfo_Field.Merge(m, src)
}
func (m *Field) XXX_Size() int {
	return m.Size()
}
func (m *Field) XXX_DiscardUnknown() {
	xxx_messageInfo_Field.DiscardUnknown(m)
}

var xxx_messageInfo_Field proto.InternalMessageInfo

type isField_Data interface {
	isField_Data()
	MarshalTo([]byte) (int, error)
	Size() int
}

type Field_StringData struct {
	StringData string `protobuf:"bytes,1,opt,name=string_data,json=stringData,proto3,oneof" json:"string_data,omitempty"`
}
type Field_IntData struct {
	IntData int64 `protobuf:"varint,2,opt,name=int_data,json=intData,proto3,oneof" json:"int_data,omitempty"`
}
type Field_BoolData struct {
	BoolData bool `protobuf:"varint,3,opt,name=bool_data,json=boolData,proto3,oneof" json:"bool_data,omitempty"`
}
type Field_BinaryData struct {
	BinaryData []byte `protobuf:"bytes,4,opt,name=binary_data,json=binaryData,proto3,oneof" json:"binary_data,omitempty"`
}

func (*Field_StringData) isField_Data() {}
func (*Field_IntData) isField_Data()    {}
func (*Field_BoolData) isField_Data()   {}
func (*Field_BinaryData) isField_Data() {}

func (m *Field) GetData() isField_Data {
	if m != nil {
		return m.Data
	}
	return nil
}

func (m *Field) GetStringData() string {
	if x, ok := m.GetData().(*Field_StringData); ok {
		return x.StringData
	}
	return ""
}

func (m *Field) GetIntData() int64 {
	if x, ok := m.GetData().(*Field_IntData); ok {
		return x.IntData
	}
	return 0
}

func (m *Field) GetBoolData() bool {
	if x, ok := m.GetData().(*Field_BoolData); ok {
		return x.BoolData
	}
	return false
}

func (m *Field) GetBinaryData() []byte {
	if x, ok := m.GetData().(*Field_BinaryData); ok {
		return x.BinaryData
	}
	return nil
}

// XXX_OneofWrappers is for the internal use of the proto package.
func (*Field) XXX_OneofWrappers() []interface{} {
	return []interface{}{
		(*Field_StringData)(nil),
		(*Field_IntData)(nil),
		(*Field_BoolData)(nil),
		(*Field_BinaryData)(nil),
	}
}

func init() {
	proto.RegisterEnum("server.uber.cadence.indexer.v1.MessageType", MessageType_name, MessageType_value)
	proto.RegisterType((*Message)(nil), "server.uber.cadence.indexer.v1.Message")
	proto.RegisterMapType((map[string]*Field)(nil), "server.uber.cadence.indexer.v1.Message.FieldsEntry")
	proto.RegisterType((*Field)(nil), "server.uber.cadence.indexer.v1.Field")
}

func init() {
	proto.RegisterFile("uber/cadence/indexer/v1/messages.proto", fileDescriptor_60256a432328b016)
}

var fileDescriptor_60256a432328b016 = []byte{
	// 495 bytes of a gzipped FileDescriptorProto
	0x1f, 0x8b, 0x08, 0x00, 0x00, 0x00, 0x00, 0x00, 0x02, 0xff, 0x8c, 0x53, 0xdd, 0x6e, 0xd3, 0x3e,
	0x14, 0xaf, 0x9b, 0xf5, 0xeb, 0x64, 0xfa, 0xab, 0x7f, 0x83, 0x20, 0xda, 0x44, 0x95, 0x4d, 0x02,
	0x55, 0x20, 0x12, 0xb5, 0xbb, 0x00, 0xb1, 0xab, 0x4d, 0x0d, 0xb4, 0x62, 0x9b, 0x50, 0x56, 0xc1,
	0x40, 0x48, 0xc1, 0x6d, 0xbc, 0x62, 0xad, 0xb1, 0x2b, 0xc7, 0x4d, 0xd7, 0xc7, 0xe0, 0x01, 0x78,
	0x1f, 0x2e, 0x79, 0x04, 0xd4, 0x27, 0x41, 0x8e, 0x33, 0xd1, 0x4e, 0x08, 0xb8, 0xb3, 0x7f, 0x9f,
	0xf2, 0xc9, 0x09, 0x3c, 0x9a, 0x8f, 0xa8, 0xf4, 0xc7, 0x24, 0xa6, 0x7c, 0x4c, 0x7d, 0xc6, 0x63,
	0x7a, 0x4d, 0xa5, 0x9f, 0x75, 0xfc, 0x84, 0xa6, 0x29, 0x99, 0xd0, 0xd4, 0x9b, 0x49, 0xa1, 0x04,
	0x6e, 0xa5, 0x54, 0x66, 0x54, 0x7a, 0x5a, 0xee, 0x15, 0x72, 0xaf, 0x90, 0x7b, 0x59, 0x67, 0xc7,
	0xdd, 0xc8, 0x21, 0x33, 0xa6, 0x33, 0xc6, 0x22, 0x49, 0x04, 0x37, 0x09, 0xfb, 0x5f, 0x2d, 0xa8,
	0x9d, 0x9a, 0x50, 0x7c, 0x06, 0xdb, 0x45, 0x7e, 0xa4, 0x96, 0x33, 0xea, 0x20, 0x17, 0xb5, 0xff,
	0xeb, 0x3e, 0xf1, 0xfe, 0x5c, 0xe2, 0x15, 0xf6, 0xe1, 0x72, 0x46, 0x43, 0x3b, 0xf9, 0x75, 0xc1,
	0xbb, 0xd0, 0x88, 0x45, 0x42, 0x18, 0x8f, 0x58, 0xec, 0x94, 0x5d, 0xd4, 0x6e, 0x84, 0x75, 0x03,
	0x0c, 0x62, 0xfc, 0x11, 0xf0, 0x42, 0xc8, 0xab, 0xcb, 0xa9, 0x58, 0x44, 0xf4, 0x9a, 0x8e, 0xe7,
	0x8a, 0x09, 0xee, 0x58, 0x2e, 0x6a, 0xdb, 0xdd, 0xa7, 0xbf, 0xad, 0x24, 0x33, 0xa6, 0xeb, 0xde,
	0x15, 0xae, 0xe0, 0xc6, 0x14, 0xfe, 0xbf, 0xb8, 0x0d, 0x61, 0x07, 0x6a, 0x19, 0x95, 0xa9, 0x8e,
	0xdc, 0x72, 0x51, 0xdb, 0x0a, 0x6f, 0xae, 0xf8, 0x35, 0x54, 0x2f, 0x19, 0x9d, 0xc6, 0xa9, 0x53,
	0x71, 0xad, 0xb6, 0xdd, 0x3d, 0xf8, 0xc7, 0xe7, 0x79, 0x2f, 0x73, 0x57, 0xc0, 0x95, 0x5c, 0x86,
	0x45, 0xc4, 0xce, 0x27, 0xb0, 0xd7, 0x60, 0xdc, 0x04, 0xeb, 0x8a, 0x2e, 0xf3, 0xb9, 0x35, 0x42,
	0x7d, 0xc4, 0x87, 0x50, 0xc9, 0xc8, 0x74, 0x4e, 0xf3, 0xe7, 0xdb, 0xdd, 0x87, 0x7f, 0x2b, 0xcb,
	0xd3, 0x42, 0xe3, 0x79, 0x51, 0x7e, 0x8e, 0xf6, 0xbf, 0x20, 0xa8, 0xe4, 0x20, 0xde, 0x03, 0x3b,
	0x55, 0x92, 0xf1, 0x49, 0x14, 0x13, 0x45, 0x4c, 0x49, 0xbf, 0x14, 0x82, 0x01, 0x7b, 0x44, 0x11,
	0xbc, 0x0b, 0x75, 0xc6, 0x95, 0xe1, 0x75, 0xa1, 0xd5, 0x2f, 0x85, 0x35, 0xc6, 0x55, 0x4e, 0x3e,
	0x80, 0xc6, 0x48, 0x88, 0xa9, 0x61, 0xf5, 0x9c, 0xeb, 0xfd, 0x52, 0x58, 0xd7, 0x50, 0x4e, 0xef,
	0x81, 0x3d, 0x62, 0x9c, 0xc8, 0xa5, 0x11, 0xe8, 0xa9, 0x6d, 0xeb, 0x78, 0x03, 0x6a, 0xc9, 0x71,
	0x15, 0xb6, 0x34, 0xf7, 0xf8, 0x02, 0xec, 0xb5, 0x6f, 0x8e, 0x1d, 0xb8, 0x7b, 0x1a, 0x9c, 0x9f,
	0x1f, 0xbd, 0x0a, 0xa2, 0xe1, 0xfb, 0x37, 0x41, 0x34, 0x38, 0x7b, 0x7b, 0x74, 0x32, 0xe8, 0x35,
	0x4b, 0xf8, 0x1e, 0xe0, 0x5b, 0x4c, 0x2f, 0xb8, 0x68, 0x22, 0x7c, 0x1f, 0xee, 0x6c, 0xe0, 0xbd,
	0xe0, 0x24, 0x18, 0x06, 0xcd, 0xf2, 0x71, 0xf0, 0x6d, 0xd5, 0x42, 0xdf, 0x57, 0x2d, 0xf4, 0x63,
	0xd5, 0x42, 0x1f, 0x9e, 0x4d, 0x98, 0xfa, 0x3c, 0x1f, 0x79, 0x63, 0x91, 0xf8, 0x1b, 0x8b, 0xec,
	0x4d, 0x28, 0xf7, 0xf3, 0xfd, 0x5d, 0xfb, 0x37, 0x0e, 0x8b, 0x63, 0xd6, 0x19, 0x55, 0x73, 0xee,
	0xe0, 0x67, 0x00, 0x00, 0x00, 0xff, 0xff, 0x5a, 0xf2, 0x16, 0x96, 0x47, 0x03, 0x00, 0x00,
}

func (m *Message) Marshal() (dAtA []byte, err error) {
	size := m.Size()
	dAtA = make([]byte, size)
	n, err := m.MarshalToSizedBuffer(dAtA[:size])
	if err != nil {
		return nil, err
	}
	return dAtA[:n], nil
}

func (m *Message) MarshalTo(dAtA []byte) (int, error) {
	size := m.Size()
	return m.MarshalToSizedBuffer(dAtA[:size])
}

func (m *Message) MarshalToSizedBuffer(dAtA []byte) (int, error) {
	i := len(dAtA)
	_ = i
	var l int
	_ = l
	if m.XXX_unrecognized != nil {
		i -= len(m.XXX_unrecognized)
		copy(dAtA[i:], m.XXX_unrecognized)
	}
	if len(m.Fields) > 0 {
		for k := range m.Fields {
			v := m.Fields[k]
			baseI := i
			if v != nil {
				{
					size, err := v.MarshalToSizedBuffer(dAtA[:i])
					if err != nil {
						return 0, err
					}
					i -= size
					i = encodeVarintMessages(dAtA, i, uint64(size))
				}
				i--
				dAtA[i] = 0x12
			}
			i -= len(k)
			copy(dAtA[i:], k)
			i = encodeVarintMessages(dAtA, i, uint64(len(k)))
			i--
			dAtA[i] = 0xa
			i = encodeVarintMessages(dAtA, i, uint64(baseI-i))
			i--
			dAtA[i] = 0x2a
		}
	}
	if m.Version != 0 {
		i = encodeVarintMessages(dAtA, i, uint64(m.Version))
		i--
		dAtA[i] = 0x20
	}
	if m.WorkflowExecution != nil {
		{
			size, err := m.WorkflowExecution.MarshalToSizedBuffer(dAtA[:i])
			if err != nil {
				return 0, err
			}
			i -= size
			i = encodeVarintMessages(dAtA, i, uint64(size))
		}
		i--
		dAtA[i] = 0x1a
	}
	if len(m.DomainId) > 0 {
		i -= len(m.DomainId)
		copy(dAtA[i:], m.DomainId)
		i = encodeVarintMessages(dAtA, i, uint64(len(m.DomainId)))
		i--
		dAtA[i] = 0x12
	}
	if m.MessageType != 0 {
		i = encodeVarintMessages(dAtA, i, uint64(m.MessageType))
		i--
		dAtA[i] = 0x8
	}
	return len(dAtA) - i, nil
}

func (m *Field) Marshal() (dAtA []byte, err error) {
	size := m.Size()
	dAtA = make([]byte, size)
	n, err := m.MarshalToSizedBuffer(dAtA[:size])
	if err != nil {
		return nil, err
	}
	return dAtA[:n], nil
}

func (m *Field) MarshalTo(dAtA []byte) (int, error) {
	size := m.Size()
	return m.MarshalToSizedBuffer(dAtA[:size])
}

func (m *Field) MarshalToSizedBuffer(dAtA []byte) (int, error) {
	i := len(dAtA)
	_ = i
	var l int
	_ = l
	if m.XXX_unrecognized != nil {
		i -= len(m.XXX_unrecognized)
		copy(dAtA[i:], m.XXX_unrecognized)
	}
	if m.Data != nil {
		{
			size := m.Data.Size()
			i -= size
			if _, err := m.Data.MarshalTo(dAtA[i:]); err != nil {
				return 0, err
			}
		}
	}
	return len(dAtA) - i, nil
}

func (m *Field_StringData) MarshalTo(dAtA []byte) (int, error) {
	size := m.Size()
	return m.MarshalToSizedBuffer(dAtA[:size])
}

func (m *Field_StringData) MarshalToSizedBuffer(dAtA []byte) (int, error) {
	i := len(dAtA)
	i -= len(m.StringData)
	copy(dAtA[i:], m.StringData)
	i = encodeVarintMessages(dAtA, i, uint64(len(m.StringData)))
	i--
	dAtA[i] = 0xa
	return len(dAtA) - i, nil
}
func (m *Field_IntData) MarshalTo(dAtA []byte) (int, error) {
	size := m.Size()
	return m.MarshalToSizedBuffer(dAtA[:size])
}

func (m *Field_IntData) MarshalToSizedBuffer(dAtA []byte) (int, error) {
	i := len(dAtA)
	i = encodeVarintMessages(dAtA, i, uint64(m.IntData))
	i--
	dAtA[i] = 0x10
	return len(dAtA) - i, nil
}
func (m *Field_BoolData) MarshalTo(dAtA []byte) (int, error) {
	size := m.Size()
	return m.MarshalToSizedBuffer(dAtA[:size])
}

func (m *Field_BoolData) MarshalToSizedBuffer(dAtA []byte) (int, error) {
	i := len(dAtA)
	i--
	if m.BoolData {
		dAtA[i] = 1
	} else {
		dAtA[i] = 0
	}
	i--
	dAtA[i] = 0x18
	return len(dAtA) - i, nil
}
func (m *Field_BinaryData) MarshalTo(dAtA []byte) (int, error) {
	size := m.Size()
	return m.MarshalToSizedBuffer(dAtA[:size])
}

func (m *Field_BinaryData) MarshalToSizedBuffer(dAtA []byte) (int, error) {
	i := len(dAtA)
	if m.BinaryData != nil {
		i -= len(m.BinaryData)
		copy(dAtA[i:], m.BinaryData)
		i = encodeVarintMessages(dAtA, i, uint64(len(m.BinaryData)))
		i--
		dAtA[i] = 0x22
	}
	return len(dAtA) - i, nil
}
func encodeVarintMessages(dAtA []byte, offset int, v uint64) int {
	offset -= sovMessages(v)
	base := offset
	for v >= 1<<7 {
		dAtA[offset] = uint8(v&0x7f | 0x80)
		v >>= 7
		offset++
	}
	dAtA[offset] = uint8(v)
	return base
}
func (m *Message) Size() (n int) {
	if m == nil {
		return 0
	}
	var l int
	_ = l
	if m.MessageType != 0 {
		n += 1 + sovMessages(uint64(m.MessageType))
	}
	l = len(m.DomainId)
	if l > 0 {
		n += 1 + l + sovMessages(uint64(l))
	}
	if m.WorkflowExecution != nil {
		l = m.WorkflowExecution.Size()
		n += 1 + l + sovMessages(uint64(l))
	}
	if m.Version != 0 {
		n += 1 + sovMessages(uint64(m.Version))
	}
	if len(m.Fields) > 0 {
		for k, v := range m.Fields {
			_ = k
			_ = v
			l = 0
			if v != nil {
				l = v.Size()
				l += 1 + sovMessages(uint64(l))
			}
			mapEntrySize := 1 + len(k) + sovMessages(uint64(len(k))) + l
			n += mapEntrySize + 1 + sovMessages(uint64(mapEntrySize))
		}
	}
	if m.XXX_unrecognized != nil {
		n += len(m.XXX_unrecognized)
	}
	return n
}

func (m *Field) Size() (n int) {
	if m == nil {
		return 0
	}
	var l int
	_ = l
	if m.Data != nil {
		n += m.Data.Size()
	}
	if m.XXX_unrecognized != nil {
		n += len(m.XXX_unrecognized)
	}
	return n
}

func (m *Field_StringData) Size() (n int) {
	if m == nil {
		return 0
	}
	var l int
	_ = l
	l = len(m.StringData)
	n += 1 + l + sovMessages(uint64(l))
	return n
}
func (m *Field_IntData) Size() (n int) {
	if m == nil {
		return 0
	}
	var l int
	_ = l
	n += 1 + sovMessages(uint64(m.IntData))
	return n
}
func (m *Field_BoolData) Size() (n int) {
	if m == nil {
		return 0
	}
	var l int
	_ = l
	n += 2
	return n
}
func (m *Field_BinaryData) Size() (n int) {
	if m == nil {
		return 0
	}
	var l int
	_ = l
	if m.BinaryData != nil {
		l = len(m.BinaryData)
		n += 1 + l + sovMessages(uint64(l))
	}
	return n
}

func sovMessages(x uint64) (n int) {
	return (math_bits.Len64(x|1) + 6) / 7
}
func sozMessages(x uint64) (n int) {
	return sovMessages(uint64((x << 1) ^ uint64((int64(x) >> 63))))
}
func (m *Message) Unmarshal(dAtA []byte) error {
	l := len(dAtA)
	iNdEx := 0
	for iNdEx < l {
		preIndex := iNdEx
		var wire uint64
		for shift := uint(0); ; shift += 7 {
			if shift >= 64 {
				return ErrIntOverflowMessages
			}
			if iNdEx >= l {
				return io.ErrUnexpectedEOF
			}
			b := dAtA[iNdEx]
			iNdEx++
			wire |= uint64(b&0x7F) << shift
			if b < 0x80 {
				break
			}
		}
		fieldNum := int32(wire >> 3)
		wireType := int(wire & 0x7)
		if wireType == 4 {
			return fmt.Errorf("proto: Message: wiretype end group for non-group")
		}
		if fieldNum <= 0 {
			return fmt.Errorf("proto: Message: illegal tag %d (wire type %d)", fieldNum, wire)
		}
		switch fieldNum {
		case 1:
			if wireType != 0 {
				return fmt.Errorf("proto: wrong wireType = %d for field MessageType", wireType)
			}
			m.MessageType = 0
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowMessages
				}
				if iNdEx >= l {
					return io.ErrUnexpectedEOF
				}
				b := dAtA[iNdEx]
				iNdEx++
				m.MessageType |= MessageType(b&0x7F) << shift
				if b < 0x80 {
					break
				}
			}
		case 2:
			if wireType != 2 {
				return fmt.Errorf("proto: wrong wireType = %d for field DomainId", wireType)
			}
			var stringLen uint64
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowMessages
				}
				if iNdEx >= l {
					return io.ErrUnexpectedEOF
				}
				b := dAtA[iNdEx]
				iNdEx++
				stringLen |= uint64(b&0x7F) << shift
				if b < 0x80 {
					break
				}
			}
			intStringLen := int(stringLen)
			if intStringLen < 0 {
				return ErrInvalidLengthMessages
			}
			postIndex := iNdEx + intStringLen
			if postIndex < 0 {
				return ErrInvalidLengthMessages
			}
			if postIndex > l {
				return io.ErrUnexpectedEOF
			}
			m.DomainId = string(dAtA[iNdEx:postIndex])
			iNdEx = postIndex
		case 3:
			if wireType != 2 {
				return fmt.Errorf("proto: wrong wireType = %d for field WorkflowExecution", wireType)
			}
			var msglen int
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowMessages
				}
				if iNdEx >= l {
					return io.ErrUnexpectedEOF
				}
				b := dAtA[iNdEx]
				iNdEx++
				msglen |= int(b&0x7F) << shift
				if b < 0x80 {
					break
				}
			}
			if msglen < 0 {
				return ErrInvalidLengthMessages
			}
			postIndex := iNdEx + msglen
			if postIndex < 0 {
				return ErrInvalidLengthMessages
			}
			if postIndex > l {
				return io.ErrUnexpectedEOF
			}
			if m.WorkflowExecution == nil {
				m.WorkflowExecution = &v1.WorkflowExecution{}
			}
			if err := m.WorkflowExecution.Unmarshal(dAtA[iNdEx:postIndex]); err != nil {
				return err
			}
			iNdEx = postIndex
		case 4:
			if wireType != 0 {
				return fmt.Errorf("proto: wrong wireType = %d for field Version", wireType)
			}
			m.Version = 0
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowMessages
				}
				if iNdEx >= l {
					return io.ErrUnexpectedEOF
				}
				b := dAtA[iNdEx]
				iNdEx++
				m.Version |= int64(b&0x7F) << shift
				if b < 0x80 {
					break
				}
			}
		case 5:
			if wireType != 2 {
				return fmt.Errorf("proto: wrong wireType = %d for field Fields", wireType)
			}
			var msglen int
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowMessages
				}
				if iNdEx >= l {
					return io.ErrUnexpectedEOF
				}
				b := dAtA[iNdEx]
				iNdEx++
				msglen |= int(b&0x7F) << shift
				if b < 0x80 {
					break
				}
			}
			if msglen < 0 {
				return ErrInvalidLengthMessages
			}
			postIndex := iNdEx + msglen
			if postIndex < 0 {
				return ErrInvalidLengthMessages
			}
			if postIndex > l {
				return io.ErrUnexpectedEOF
			}
			if m.Fields == nil {
				m.Fields = make(map[string]*Field)
			}
			var mapkey string
			var mapvalue *Field
			for iNdEx < postIndex {
				entryPreIndex := iNdEx
				var wire uint64
				for shift := uint(0); ; shift += 7 {
					if shift >= 64 {
						return ErrIntOverflowMessages
					}
					if iNdEx >= l {
						return io.ErrUnexpectedEOF
					}
					b := dAtA[iNdEx]
					iNdEx++
					wire |= uint64(b&0x7F) << shift
					if b < 0x80 {
						break
					}
				}
				fieldNum := int32(wire >> 3)
				if fieldNum == 1 {
					var stringLenmapkey uint64
					for shift := uint(0); ; shift += 7 {
						if shift >= 64 {
							return ErrIntOverflowMessages
						}
						if iNdEx >= l {
							return io.ErrUnexpectedEOF
						}
						b := dAtA[iNdEx]
						iNdEx++
						stringLenmapkey |= uint64(b&0x7F) << shift
						if b < 0x80 {
							break
						}
					}
					intStringLenmapkey := int(stringLenmapkey)
					if intStringLenmapkey < 0 {
						return ErrInvalidLengthMessages
					}
					postStringIndexmapkey := iNdEx + intStringLenmapkey
					if postStringIndexmapkey < 0 {
						return ErrInvalidLengthMessages
					}
					if postStringIndexmapkey > l {
						return io.ErrUnexpectedEOF
					}
					mapkey = string(dAtA[iNdEx:postStringIndexmapkey])
					iNdEx = postStringIndexmapkey
				} else if fieldNum == 2 {
					var mapmsglen int
					for shift := uint(0); ; shift += 7 {
						if shift >= 64 {
							return ErrIntOverflowMessages
						}
						if iNdEx >= l {
							return io.ErrUnexpectedEOF
						}
						b := dAtA[iNdEx]
						iNdEx++
						mapmsglen |= int(b&0x7F) << shift
						if b < 0x80 {
							break
						}
					}
					if mapmsglen < 0 {
						return ErrInvalidLengthMessages
					}
					postmsgIndex := iNdEx + mapmsglen
					if postmsgIndex < 0 {
						return ErrInvalidLengthMessages
					}
					if postmsgIndex > l {
						return io.ErrUnexpectedEOF
					}
					mapvalue = &Field{}
					if err := mapvalue.Unmarshal(dAtA[iNdEx:postmsgIndex]); err != nil {
						return err
					}
					iNdEx = postmsgIndex
				} else {
					iNdEx = entryPreIndex
					skippy, err := skipMessages(dAtA[iNdEx:])
					if err != nil {
						return err
					}
					if (skippy < 0) || (iNdEx+skippy) < 0 {
						return ErrInvalidLengthMessages
					}
					if (iNdEx + skippy) > postIndex {
						return io.ErrUnexpectedEOF
					}
					iNdEx += skippy
				}
			}
			m.Fields[mapkey] = mapvalue
			iNdEx = postIndex
		default:
			iNdEx = preIndex
			skippy, err := skipMessages(dAtA[iNdEx:])
			if err != nil {
				return err
			}
			if (skippy < 0) || (iNdEx+skippy) < 0 {
				return ErrInvalidLengthMessages
			}
			if (iNdEx + skippy) > l {
				return io.ErrUnexpectedEOF
			}
			m.XXX_unrecognized = append(m.XXX_unrecognized, dAtA[iNdEx:iNdEx+skippy]...)
			iNdEx += skippy
		}
	}

	if iNdEx > l {
		return io.ErrUnexpectedEOF
	}
	return nil
}
func (m *Field) Unmarshal(dAtA []byte) error {
	l := len(dAtA)
	iNdEx := 0
	for iNdEx < l {
		preIndex := iNdEx
		var wire uint64
		for shift := uint(0); ; shift += 7 {
			if shift >= 64 {
				return ErrIntOverflowMessages
			}
			if iNdEx >= l {
				return io.ErrUnexpectedEOF
			}
			b := dAtA[iNdEx]
			iNdEx++
			wire |= uint64(b&0x7F) << shift
			if b < 0x80 {
				break
			}
		}
		fieldNum := int32(wire >> 3)
		wireType := int(wire & 0x7)
		if wireType == 4 {
			return fmt.Errorf("proto: Field: wiretype end group for non-group")
		}
		if fieldNum <= 0 {
			return fmt.Errorf("proto: Field: illegal tag %d (wire type %d)", fieldNum, wire)
		}
		switch fieldNum {
		case 1:
			if wireType != 2 {
				return fmt.Errorf("proto: wrong wireType = %d for field StringData", wireType)
			}
			var stringLen uint64
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowMessages
				}
				if iNdEx >= l {
					return io.ErrUnexpectedEOF
				}
				b := dAtA[iNdEx]
				iNdEx++
				stringLen |= uint64(b&0x7F) << shift
				if b < 0x80 {
					break
				}
			}
			intStringLen := int(stringLen)
			if intStringLen < 0 {
				return ErrInvalidLengthMessages
			}
			postIndex := iNdEx + intStringLen
			if postIndex < 0 {
				return ErrInvalidLengthMessages
			}
			if postIndex > l {
				return io.ErrUnexpectedEOF
			}
			m.Data = &Field_StringData{string(dAtA[iNdEx:postIndex])}
			iNdEx = postIndex
		case 2:
			if wireType != 0 {
				return fmt.Errorf("proto: wrong wireType = %d for field IntData", wireType)
			}
			var v int64
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowMessages
				}
				if iNdEx >= l {
					return io.ErrUnexpectedEOF
				}
				b := dAtA[iNdEx]
				iNdEx++
				v |= int64(b&0x7F) << shift
				if b < 0x80 {
					break
				}
			}
			m.Data = &Field_IntData{v}
		case 3:
			if wireType != 0 {
				return fmt.Errorf("proto: wrong wireType = %d for field BoolData", wireType)
			}
			var v int
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowMessages
				}
				if iNdEx >= l {
					return io.ErrUnexpectedEOF
				}
				b := dAtA[iNdEx]
				iNdEx++
				v |= int(b&0x7F) << shift
				if b < 0x80 {
					break
				}
			}
			b := bool(v != 0)
			m.Data = &Field_BoolData{b}
		case 4:
			if wireType != 2 {
				return fmt.Errorf("proto: wrong wireType = %d for field BinaryData", wireType)
			}
			var byteLen int
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowMessages
				}
				if iNdEx >= l {
					return io.ErrUnexpectedEOF
				}
				b := dAtA[iNdEx]
				iNdEx++
				byteLen |= int(b&0x7F) << shift
				if b < 0x80 {
					break
				}
			}
			if byteLen < 0 {
				return ErrInvalidLengthMessages
			}
			postIndex := iNdEx + byteLen
			if postIndex < 0 {
				return ErrInvalidLengthMessages
			}
			if postIndex > l {
				return io.ErrUnexpectedEOF
			}
			v := make([]byte, postIndex-iNdEx)
			copy(v, dAtA[iNdEx:postIndex])
			m.Data = &Field_BinaryData{v}
			iNdEx = postIndex
		default:
			iNdEx = preIndex
			skippy, err := skipMessages(dAtA[iNdEx:])
			if err != nil {
				return err
			}
			if (skippy < 0) || (iNdEx+skippy) < 0 {
				return ErrInvalidLengthMessages
			}
			if (iNdEx + skippy) > l {
				return io.ErrUnexpectedEOF
			}
			m.XXX_unrecognized = append(m.XXX_unrecognized, dAtA[iNdEx:iNdEx+skippy]...)
			iNdEx += skippy
		}
	}

	if iNdEx > l {
		return io.ErrUnexpectedEOF
	}
	return nil
}
func skipMessages(dAtA []byte) (n int, err error) {
	l := len(dAtA)
	iNdEx := 0
	depth := 0
	for iNdEx < l {
		var wire uint64
		for shift := uint(0); ; shift += 7 {
			if shift >= 64 {
				return 0, ErrIntOverflowMessages
			}
			if iNdEx >= l {
				return 0, io.ErrUnexpectedEOF
			}
			b := dAtA[iNdEx]
			iNdEx++
			wire |= (uint64(b) & 0x7F) << shift
			if b < 0x80 {
				break
			}
		}
		wireType := int(wire & 0x7)
		switch wireType {
		case 0:
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return 0, ErrIntOverflowMessages
				}
				if iNdEx >= l {
					return 0, io.ErrUnexpectedEOF
				}
				iNdEx++
				if dAtA[iNdEx-1] < 0x80 {
					break
				}
			}
		case 1:
			iNdEx += 8
		case 2:
			var length int
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return 0, ErrIntOverflowMessages
				}
				if iNdEx >= l {
					return 0, io.ErrUnexpectedEOF
				}
				b := dAtA[iNdEx]
				iNdEx++
				length |= (int(b) & 0x7F) << shift
				if b < 0x80 {
					break
				}
			}
			if length < 0 {
				return 0, ErrInvalidLengthMessages
			}
			iNdEx += length
		case 3:
			depth++
		case 4:
			if depth == 0 {
				return 0, ErrUnexpectedEndOfGroupMessages
			}
			depth--
		case 5:
			iNdEx += 4
		default:
			return 0, fmt.Errorf("proto: illegal wireType %d", wireType)
		}
		if iNdEx < 0 {
			return 0, ErrInvalidLengthMessages
		}
		if depth == 0 {
			return iNdEx, nil
		}
	}
	return 0, io.ErrUnexpectedEOF
}

var (
	ErrInvalidLengthMessages        = fmt.Errorf("proto: negative length found during unmarshaling")
	ErrIntOverflowMessages          = fmt.Errorf("proto: integer overflow")
	ErrUnexpectedEndOfGroupMessages = fmt.Errorf("proto: unexpected end of group")
)
