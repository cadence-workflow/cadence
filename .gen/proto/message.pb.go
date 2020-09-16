// The MIT License (MIT)
// 
// Copyright (c) 2017-2020 Uber Technologies Inc.
// 
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

// Code generated by protoc-gen-go. DO NOT EDIT.
// versions:
// 	protoc-gen-go v1.25.0
// 	protoc        v3.13.0
// source: message.proto

package proto

import (
	proto "github.com/golang/protobuf/proto"
	duration "github.com/golang/protobuf/ptypes/duration"
	timestamp "github.com/golang/protobuf/ptypes/timestamp"
	protoreflect "google.golang.org/protobuf/reflect/protoreflect"
	protoimpl "google.golang.org/protobuf/runtime/protoimpl"
	reflect "reflect"
	sync "sync"
)

const (
	// Verify that this generated code is sufficiently up-to-date.
	_ = protoimpl.EnforceVersion(20 - protoimpl.MinVersion)
	// Verify that runtime/protoimpl is sufficiently up-to-date.
	_ = protoimpl.EnforceVersion(protoimpl.MaxVersion - 20)
)

// This is a compile-time assertion that a sufficiently up-to-date version
// of the legacy proto package is being used.
const _ = proto.ProtoPackageIsVersion4

type ArchivalState int32

const (
	ArchivalState_ARCHIVAL_STATE_UNSPECIFIED ArchivalState = 0
	ArchivalState_ARCHIVAL_STATE_DISABLED    ArchivalState = 1
	ArchivalState_ARCHIVAL_STATE_ENABLED     ArchivalState = 2
)

// Enum value maps for ArchivalState.
var (
	ArchivalState_name = map[int32]string{
		0: "ARCHIVAL_STATE_UNSPECIFIED",
		1: "ARCHIVAL_STATE_DISABLED",
		2: "ARCHIVAL_STATE_ENABLED",
	}
	ArchivalState_value = map[string]int32{
		"ARCHIVAL_STATE_UNSPECIFIED": 0,
		"ARCHIVAL_STATE_DISABLED":    1,
		"ARCHIVAL_STATE_ENABLED":     2,
	}
)

func (x ArchivalState) Enum() *ArchivalState {
	p := new(ArchivalState)
	*p = x
	return p
}

func (x ArchivalState) String() string {
	return protoimpl.X.EnumStringOf(x.Descriptor(), protoreflect.EnumNumber(x))
}

func (ArchivalState) Descriptor() protoreflect.EnumDescriptor {
	return file_message_proto_enumTypes[0].Descriptor()
}

func (ArchivalState) Type() protoreflect.EnumType {
	return &file_message_proto_enumTypes[0]
}

func (x ArchivalState) Number() protoreflect.EnumNumber {
	return protoreflect.EnumNumber(x)
}

// Deprecated: Use ArchivalState.Descriptor instead.
func (ArchivalState) EnumDescriptor() ([]byte, []int) {
	return file_message_proto_rawDescGZIP(), []int{0}
}

type DomainState int32

const (
	DomainState_DOMAIN_STATE_UNSPECIFIED DomainState = 0
	DomainState_DOMAIN_STATE_REGISTERED  DomainState = 1
	DomainState_DOMAIN_STATE_DEPRECATED  DomainState = 2
	DomainState_DOMAIN_STATE_DELETED     DomainState = 3
)

// Enum value maps for DomainState.
var (
	DomainState_name = map[int32]string{
		0: "DOMAIN_STATE_UNSPECIFIED",
		1: "DOMAIN_STATE_REGISTERED",
		2: "DOMAIN_STATE_DEPRECATED",
		3: "DOMAIN_STATE_DELETED",
	}
	DomainState_value = map[string]int32{
		"DOMAIN_STATE_UNSPECIFIED": 0,
		"DOMAIN_STATE_REGISTERED":  1,
		"DOMAIN_STATE_DEPRECATED":  2,
		"DOMAIN_STATE_DELETED":     3,
	}
)

func (x DomainState) Enum() *DomainState {
	p := new(DomainState)
	*p = x
	return p
}

func (x DomainState) String() string {
	return protoimpl.X.EnumStringOf(x.Descriptor(), protoreflect.EnumNumber(x))
}

func (DomainState) Descriptor() protoreflect.EnumDescriptor {
	return file_message_proto_enumTypes[1].Descriptor()
}

func (DomainState) Type() protoreflect.EnumType {
	return &file_message_proto_enumTypes[1]
}

func (x DomainState) Number() protoreflect.EnumNumber {
	return protoreflect.EnumNumber(x)
}

// Deprecated: Use DomainState.Descriptor instead.
func (DomainState) EnumDescriptor() ([]byte, []int) {
	return file_message_proto_rawDescGZIP(), []int{1}
}

type DomainDetail struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Info                        *DomainInfo              `protobuf:"bytes,1,opt,name=info,proto3" json:"info,omitempty"`
	Config                      *DomainConfig            `protobuf:"bytes,2,opt,name=config,proto3" json:"config,omitempty"`
	ReplicationConfig           *DomainReplicationConfig `protobuf:"bytes,3,opt,name=replication_config,json=replicationConfig,proto3" json:"replication_config,omitempty"`
	ConfigVersion               int64                    `protobuf:"varint,4,opt,name=config_version,json=configVersion,proto3" json:"config_version,omitempty"`
	FailoverNotificationVersion int64                    `protobuf:"varint,5,opt,name=failover_notification_version,json=failoverNotificationVersion,proto3" json:"failover_notification_version,omitempty"`
	FailoverVersion             int64                    `protobuf:"varint,6,opt,name=failover_version,json=failoverVersion,proto3" json:"failover_version,omitempty"`
	FailoverEndTime             *timestamp.Timestamp     `protobuf:"bytes,7,opt,name=failover_end_time,json=failoverEndTime,proto3" json:"failover_end_time,omitempty"`
}

func (x *DomainDetail) Reset() {
	*x = DomainDetail{}
	if protoimpl.UnsafeEnabled {
		mi := &file_message_proto_msgTypes[0]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *DomainDetail) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*DomainDetail) ProtoMessage() {}

func (x *DomainDetail) ProtoReflect() protoreflect.Message {
	mi := &file_message_proto_msgTypes[0]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use DomainDetail.ProtoReflect.Descriptor instead.
func (*DomainDetail) Descriptor() ([]byte, []int) {
	return file_message_proto_rawDescGZIP(), []int{0}
}

func (x *DomainDetail) GetInfo() *DomainInfo {
	if x != nil {
		return x.Info
	}
	return nil
}

func (x *DomainDetail) GetConfig() *DomainConfig {
	if x != nil {
		return x.Config
	}
	return nil
}

func (x *DomainDetail) GetReplicationConfig() *DomainReplicationConfig {
	if x != nil {
		return x.ReplicationConfig
	}
	return nil
}

func (x *DomainDetail) GetConfigVersion() int64 {
	if x != nil {
		return x.ConfigVersion
	}
	return 0
}

func (x *DomainDetail) GetFailoverNotificationVersion() int64 {
	if x != nil {
		return x.FailoverNotificationVersion
	}
	return 0
}

func (x *DomainDetail) GetFailoverVersion() int64 {
	if x != nil {
		return x.FailoverVersion
	}
	return 0
}

func (x *DomainDetail) GetFailoverEndTime() *timestamp.Timestamp {
	if x != nil {
		return x.FailoverEndTime
	}
	return nil
}

type DomainInfo struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Id          string            `protobuf:"bytes,1,opt,name=id,proto3" json:"id,omitempty"`
	State       DomainState       `protobuf:"varint,2,opt,name=state,proto3,enum=DomainState" json:"state,omitempty"`
	Name        string            `protobuf:"bytes,3,opt,name=name,proto3" json:"name,omitempty"`
	Description string            `protobuf:"bytes,4,opt,name=description,proto3" json:"description,omitempty"`
	Owner       string            `protobuf:"bytes,5,opt,name=owner,proto3" json:"owner,omitempty"`
	Data        map[string]string `protobuf:"bytes,6,rep,name=data,proto3" json:"data,omitempty" protobuf_key:"bytes,1,opt,name=key,proto3" protobuf_val:"bytes,2,opt,name=value,proto3"`
}

func (x *DomainInfo) Reset() {
	*x = DomainInfo{}
	if protoimpl.UnsafeEnabled {
		mi := &file_message_proto_msgTypes[1]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *DomainInfo) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*DomainInfo) ProtoMessage() {}

func (x *DomainInfo) ProtoReflect() protoreflect.Message {
	mi := &file_message_proto_msgTypes[1]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use DomainInfo.ProtoReflect.Descriptor instead.
func (*DomainInfo) Descriptor() ([]byte, []int) {
	return file_message_proto_rawDescGZIP(), []int{1}
}

func (x *DomainInfo) GetId() string {
	if x != nil {
		return x.Id
	}
	return ""
}

func (x *DomainInfo) GetState() DomainState {
	if x != nil {
		return x.State
	}
	return DomainState_DOMAIN_STATE_UNSPECIFIED
}

func (x *DomainInfo) GetName() string {
	if x != nil {
		return x.Name
	}
	return ""
}

func (x *DomainInfo) GetDescription() string {
	if x != nil {
		return x.Description
	}
	return ""
}

func (x *DomainInfo) GetOwner() string {
	if x != nil {
		return x.Owner
	}
	return ""
}

func (x *DomainInfo) GetData() map[string]string {
	if x != nil {
		return x.Data
	}
	return nil
}

type DomainReplicationConfig struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	ActiveClusterName string   `protobuf:"bytes,1,opt,name=active_cluster_name,json=activeClusterName,proto3" json:"active_cluster_name,omitempty"`
	Clusters          []string `protobuf:"bytes,2,rep,name=clusters,proto3" json:"clusters,omitempty"`
}

func (x *DomainReplicationConfig) Reset() {
	*x = DomainReplicationConfig{}
	if protoimpl.UnsafeEnabled {
		mi := &file_message_proto_msgTypes[2]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *DomainReplicationConfig) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*DomainReplicationConfig) ProtoMessage() {}

func (x *DomainReplicationConfig) ProtoReflect() protoreflect.Message {
	mi := &file_message_proto_msgTypes[2]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use DomainReplicationConfig.ProtoReflect.Descriptor instead.
func (*DomainReplicationConfig) Descriptor() ([]byte, []int) {
	return file_message_proto_rawDescGZIP(), []int{2}
}

func (x *DomainReplicationConfig) GetActiveClusterName() string {
	if x != nil {
		return x.ActiveClusterName
	}
	return ""
}

func (x *DomainReplicationConfig) GetClusters() []string {
	if x != nil {
		return x.Clusters
	}
	return nil
}

type DomainConfig struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Retention               *duration.Duration `protobuf:"bytes,1,opt,name=retention,proto3" json:"retention,omitempty"`
	ArchivalBucket          string             `protobuf:"bytes,2,opt,name=archival_bucket,json=archivalBucket,proto3" json:"archival_bucket,omitempty"`
	BadBinaries             *BadBinaries       `protobuf:"bytes,3,opt,name=bad_binaries,json=badBinaries,proto3" json:"bad_binaries,omitempty"`
	HistoryArchivalState    ArchivalState      `protobuf:"varint,4,opt,name=history_archival_state,json=historyArchivalState,proto3,enum=ArchivalState" json:"history_archival_state,omitempty"`
	HistoryArchivalUri      string             `protobuf:"bytes,5,opt,name=history_archival_uri,json=historyArchivalUri,proto3" json:"history_archival_uri,omitempty"`
	VisibilityArchivalState ArchivalState      `protobuf:"varint,6,opt,name=visibility_archival_state,json=visibilityArchivalState,proto3,enum=ArchivalState" json:"visibility_archival_state,omitempty"`
	VisibilityArchivalUri   string             `protobuf:"bytes,7,opt,name=visibility_archival_uri,json=visibilityArchivalUri,proto3" json:"visibility_archival_uri,omitempty"`
}

func (x *DomainConfig) Reset() {
	*x = DomainConfig{}
	if protoimpl.UnsafeEnabled {
		mi := &file_message_proto_msgTypes[3]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *DomainConfig) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*DomainConfig) ProtoMessage() {}

func (x *DomainConfig) ProtoReflect() protoreflect.Message {
	mi := &file_message_proto_msgTypes[3]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use DomainConfig.ProtoReflect.Descriptor instead.
func (*DomainConfig) Descriptor() ([]byte, []int) {
	return file_message_proto_rawDescGZIP(), []int{3}
}

func (x *DomainConfig) GetRetention() *duration.Duration {
	if x != nil {
		return x.Retention
	}
	return nil
}

func (x *DomainConfig) GetArchivalBucket() string {
	if x != nil {
		return x.ArchivalBucket
	}
	return ""
}

func (x *DomainConfig) GetBadBinaries() *BadBinaries {
	if x != nil {
		return x.BadBinaries
	}
	return nil
}

func (x *DomainConfig) GetHistoryArchivalState() ArchivalState {
	if x != nil {
		return x.HistoryArchivalState
	}
	return ArchivalState_ARCHIVAL_STATE_UNSPECIFIED
}

func (x *DomainConfig) GetHistoryArchivalUri() string {
	if x != nil {
		return x.HistoryArchivalUri
	}
	return ""
}

func (x *DomainConfig) GetVisibilityArchivalState() ArchivalState {
	if x != nil {
		return x.VisibilityArchivalState
	}
	return ArchivalState_ARCHIVAL_STATE_UNSPECIFIED
}

func (x *DomainConfig) GetVisibilityArchivalUri() string {
	if x != nil {
		return x.VisibilityArchivalUri
	}
	return ""
}

type BadBinaries struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Binaries map[string]*BadBinaryInfo `protobuf:"bytes,1,rep,name=binaries,proto3" json:"binaries,omitempty" protobuf_key:"bytes,1,opt,name=key,proto3" protobuf_val:"bytes,2,opt,name=value,proto3"`
}

func (x *BadBinaries) Reset() {
	*x = BadBinaries{}
	if protoimpl.UnsafeEnabled {
		mi := &file_message_proto_msgTypes[4]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *BadBinaries) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*BadBinaries) ProtoMessage() {}

func (x *BadBinaries) ProtoReflect() protoreflect.Message {
	mi := &file_message_proto_msgTypes[4]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use BadBinaries.ProtoReflect.Descriptor instead.
func (*BadBinaries) Descriptor() ([]byte, []int) {
	return file_message_proto_rawDescGZIP(), []int{4}
}

func (x *BadBinaries) GetBinaries() map[string]*BadBinaryInfo {
	if x != nil {
		return x.Binaries
	}
	return nil
}

type BadBinaryInfo struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Reason     string               `protobuf:"bytes,1,opt,name=reason,proto3" json:"reason,omitempty"`
	Operator   string               `protobuf:"bytes,2,opt,name=operator,proto3" json:"operator,omitempty"`
	CreateTime *timestamp.Timestamp `protobuf:"bytes,3,opt,name=create_time,json=createTime,proto3" json:"create_time,omitempty"`
}

func (x *BadBinaryInfo) Reset() {
	*x = BadBinaryInfo{}
	if protoimpl.UnsafeEnabled {
		mi := &file_message_proto_msgTypes[5]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *BadBinaryInfo) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*BadBinaryInfo) ProtoMessage() {}

func (x *BadBinaryInfo) ProtoReflect() protoreflect.Message {
	mi := &file_message_proto_msgTypes[5]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use BadBinaryInfo.ProtoReflect.Descriptor instead.
func (*BadBinaryInfo) Descriptor() ([]byte, []int) {
	return file_message_proto_rawDescGZIP(), []int{5}
}

func (x *BadBinaryInfo) GetReason() string {
	if x != nil {
		return x.Reason
	}
	return ""
}

func (x *BadBinaryInfo) GetOperator() string {
	if x != nil {
		return x.Operator
	}
	return ""
}

func (x *BadBinaryInfo) GetCreateTime() *timestamp.Timestamp {
	if x != nil {
		return x.CreateTime
	}
	return nil
}

var File_message_proto protoreflect.FileDescriptor

var file_message_proto_rawDesc = []byte{
	0x0a, 0x0d, 0x6d, 0x65, 0x73, 0x73, 0x61, 0x67, 0x65, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x1a,
	0x1e, 0x67, 0x6f, 0x6f, 0x67, 0x6c, 0x65, 0x2f, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x62, 0x75, 0x66,
	0x2f, 0x64, 0x75, 0x72, 0x61, 0x74, 0x69, 0x6f, 0x6e, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x1a,
	0x1f, 0x67, 0x6f, 0x6f, 0x67, 0x6c, 0x65, 0x2f, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x62, 0x75, 0x66,
	0x2f, 0x74, 0x69, 0x6d, 0x65, 0x73, 0x74, 0x61, 0x6d, 0x70, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f,
	0x1a, 0x0a, 0x67, 0x6f, 0x67, 0x6f, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x22, 0x83, 0x03, 0x0a,
	0x0c, 0x44, 0x6f, 0x6d, 0x61, 0x69, 0x6e, 0x44, 0x65, 0x74, 0x61, 0x69, 0x6c, 0x12, 0x1f, 0x0a,
	0x04, 0x69, 0x6e, 0x66, 0x6f, 0x18, 0x01, 0x20, 0x01, 0x28, 0x0b, 0x32, 0x0b, 0x2e, 0x44, 0x6f,
	0x6d, 0x61, 0x69, 0x6e, 0x49, 0x6e, 0x66, 0x6f, 0x52, 0x04, 0x69, 0x6e, 0x66, 0x6f, 0x12, 0x25,
	0x0a, 0x06, 0x63, 0x6f, 0x6e, 0x66, 0x69, 0x67, 0x18, 0x02, 0x20, 0x01, 0x28, 0x0b, 0x32, 0x0d,
	0x2e, 0x44, 0x6f, 0x6d, 0x61, 0x69, 0x6e, 0x43, 0x6f, 0x6e, 0x66, 0x69, 0x67, 0x52, 0x06, 0x63,
	0x6f, 0x6e, 0x66, 0x69, 0x67, 0x12, 0x47, 0x0a, 0x12, 0x72, 0x65, 0x70, 0x6c, 0x69, 0x63, 0x61,
	0x74, 0x69, 0x6f, 0x6e, 0x5f, 0x63, 0x6f, 0x6e, 0x66, 0x69, 0x67, 0x18, 0x03, 0x20, 0x01, 0x28,
	0x0b, 0x32, 0x18, 0x2e, 0x44, 0x6f, 0x6d, 0x61, 0x69, 0x6e, 0x52, 0x65, 0x70, 0x6c, 0x69, 0x63,
	0x61, 0x74, 0x69, 0x6f, 0x6e, 0x43, 0x6f, 0x6e, 0x66, 0x69, 0x67, 0x52, 0x11, 0x72, 0x65, 0x70,
	0x6c, 0x69, 0x63, 0x61, 0x74, 0x69, 0x6f, 0x6e, 0x43, 0x6f, 0x6e, 0x66, 0x69, 0x67, 0x12, 0x25,
	0x0a, 0x0e, 0x63, 0x6f, 0x6e, 0x66, 0x69, 0x67, 0x5f, 0x76, 0x65, 0x72, 0x73, 0x69, 0x6f, 0x6e,
	0x18, 0x04, 0x20, 0x01, 0x28, 0x03, 0x52, 0x0d, 0x63, 0x6f, 0x6e, 0x66, 0x69, 0x67, 0x56, 0x65,
	0x72, 0x73, 0x69, 0x6f, 0x6e, 0x12, 0x42, 0x0a, 0x1d, 0x66, 0x61, 0x69, 0x6c, 0x6f, 0x76, 0x65,
	0x72, 0x5f, 0x6e, 0x6f, 0x74, 0x69, 0x66, 0x69, 0x63, 0x61, 0x74, 0x69, 0x6f, 0x6e, 0x5f, 0x76,
	0x65, 0x72, 0x73, 0x69, 0x6f, 0x6e, 0x18, 0x05, 0x20, 0x01, 0x28, 0x03, 0x52, 0x1b, 0x66, 0x61,
	0x69, 0x6c, 0x6f, 0x76, 0x65, 0x72, 0x4e, 0x6f, 0x74, 0x69, 0x66, 0x69, 0x63, 0x61, 0x74, 0x69,
	0x6f, 0x6e, 0x56, 0x65, 0x72, 0x73, 0x69, 0x6f, 0x6e, 0x12, 0x29, 0x0a, 0x10, 0x66, 0x61, 0x69,
	0x6c, 0x6f, 0x76, 0x65, 0x72, 0x5f, 0x76, 0x65, 0x72, 0x73, 0x69, 0x6f, 0x6e, 0x18, 0x06, 0x20,
	0x01, 0x28, 0x03, 0x52, 0x0f, 0x66, 0x61, 0x69, 0x6c, 0x6f, 0x76, 0x65, 0x72, 0x56, 0x65, 0x72,
	0x73, 0x69, 0x6f, 0x6e, 0x12, 0x4c, 0x0a, 0x11, 0x66, 0x61, 0x69, 0x6c, 0x6f, 0x76, 0x65, 0x72,
	0x5f, 0x65, 0x6e, 0x64, 0x5f, 0x74, 0x69, 0x6d, 0x65, 0x18, 0x07, 0x20, 0x01, 0x28, 0x0b, 0x32,
	0x1a, 0x2e, 0x67, 0x6f, 0x6f, 0x67, 0x6c, 0x65, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x62, 0x75,
	0x66, 0x2e, 0x54, 0x69, 0x6d, 0x65, 0x73, 0x74, 0x61, 0x6d, 0x70, 0x42, 0x04, 0x90, 0xdf, 0x1f,
	0x01, 0x52, 0x0f, 0x66, 0x61, 0x69, 0x6c, 0x6f, 0x76, 0x65, 0x72, 0x45, 0x6e, 0x64, 0x54, 0x69,
	0x6d, 0x65, 0x22, 0xf0, 0x01, 0x0a, 0x0a, 0x44, 0x6f, 0x6d, 0x61, 0x69, 0x6e, 0x49, 0x6e, 0x66,
	0x6f, 0x12, 0x0e, 0x0a, 0x02, 0x69, 0x64, 0x18, 0x01, 0x20, 0x01, 0x28, 0x09, 0x52, 0x02, 0x69,
	0x64, 0x12, 0x22, 0x0a, 0x05, 0x73, 0x74, 0x61, 0x74, 0x65, 0x18, 0x02, 0x20, 0x01, 0x28, 0x0e,
	0x32, 0x0c, 0x2e, 0x44, 0x6f, 0x6d, 0x61, 0x69, 0x6e, 0x53, 0x74, 0x61, 0x74, 0x65, 0x52, 0x05,
	0x73, 0x74, 0x61, 0x74, 0x65, 0x12, 0x12, 0x0a, 0x04, 0x6e, 0x61, 0x6d, 0x65, 0x18, 0x03, 0x20,
	0x01, 0x28, 0x09, 0x52, 0x04, 0x6e, 0x61, 0x6d, 0x65, 0x12, 0x20, 0x0a, 0x0b, 0x64, 0x65, 0x73,
	0x63, 0x72, 0x69, 0x70, 0x74, 0x69, 0x6f, 0x6e, 0x18, 0x04, 0x20, 0x01, 0x28, 0x09, 0x52, 0x0b,
	0x64, 0x65, 0x73, 0x63, 0x72, 0x69, 0x70, 0x74, 0x69, 0x6f, 0x6e, 0x12, 0x14, 0x0a, 0x05, 0x6f,
	0x77, 0x6e, 0x65, 0x72, 0x18, 0x05, 0x20, 0x01, 0x28, 0x09, 0x52, 0x05, 0x6f, 0x77, 0x6e, 0x65,
	0x72, 0x12, 0x29, 0x0a, 0x04, 0x64, 0x61, 0x74, 0x61, 0x18, 0x06, 0x20, 0x03, 0x28, 0x0b, 0x32,
	0x15, 0x2e, 0x44, 0x6f, 0x6d, 0x61, 0x69, 0x6e, 0x49, 0x6e, 0x66, 0x6f, 0x2e, 0x44, 0x61, 0x74,
	0x61, 0x45, 0x6e, 0x74, 0x72, 0x79, 0x52, 0x04, 0x64, 0x61, 0x74, 0x61, 0x1a, 0x37, 0x0a, 0x09,
	0x44, 0x61, 0x74, 0x61, 0x45, 0x6e, 0x74, 0x72, 0x79, 0x12, 0x10, 0x0a, 0x03, 0x6b, 0x65, 0x79,
	0x18, 0x01, 0x20, 0x01, 0x28, 0x09, 0x52, 0x03, 0x6b, 0x65, 0x79, 0x12, 0x14, 0x0a, 0x05, 0x76,
	0x61, 0x6c, 0x75, 0x65, 0x18, 0x02, 0x20, 0x01, 0x28, 0x09, 0x52, 0x05, 0x76, 0x61, 0x6c, 0x75,
	0x65, 0x3a, 0x02, 0x38, 0x01, 0x22, 0x65, 0x0a, 0x17, 0x44, 0x6f, 0x6d, 0x61, 0x69, 0x6e, 0x52,
	0x65, 0x70, 0x6c, 0x69, 0x63, 0x61, 0x74, 0x69, 0x6f, 0x6e, 0x43, 0x6f, 0x6e, 0x66, 0x69, 0x67,
	0x12, 0x2e, 0x0a, 0x13, 0x61, 0x63, 0x74, 0x69, 0x76, 0x65, 0x5f, 0x63, 0x6c, 0x75, 0x73, 0x74,
	0x65, 0x72, 0x5f, 0x6e, 0x61, 0x6d, 0x65, 0x18, 0x01, 0x20, 0x01, 0x28, 0x09, 0x52, 0x11, 0x61,
	0x63, 0x74, 0x69, 0x76, 0x65, 0x43, 0x6c, 0x75, 0x73, 0x74, 0x65, 0x72, 0x4e, 0x61, 0x6d, 0x65,
	0x12, 0x1a, 0x0a, 0x08, 0x63, 0x6c, 0x75, 0x73, 0x74, 0x65, 0x72, 0x73, 0x18, 0x02, 0x20, 0x03,
	0x28, 0x09, 0x52, 0x08, 0x63, 0x6c, 0x75, 0x73, 0x74, 0x65, 0x72, 0x73, 0x22, 0xa3, 0x03, 0x0a,
	0x0c, 0x44, 0x6f, 0x6d, 0x61, 0x69, 0x6e, 0x43, 0x6f, 0x6e, 0x66, 0x69, 0x67, 0x12, 0x3d, 0x0a,
	0x09, 0x72, 0x65, 0x74, 0x65, 0x6e, 0x74, 0x69, 0x6f, 0x6e, 0x18, 0x01, 0x20, 0x01, 0x28, 0x0b,
	0x32, 0x19, 0x2e, 0x67, 0x6f, 0x6f, 0x67, 0x6c, 0x65, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x62,
	0x75, 0x66, 0x2e, 0x44, 0x75, 0x72, 0x61, 0x74, 0x69, 0x6f, 0x6e, 0x42, 0x04, 0x98, 0xdf, 0x1f,
	0x01, 0x52, 0x09, 0x72, 0x65, 0x74, 0x65, 0x6e, 0x74, 0x69, 0x6f, 0x6e, 0x12, 0x27, 0x0a, 0x0f,
	0x61, 0x72, 0x63, 0x68, 0x69, 0x76, 0x61, 0x6c, 0x5f, 0x62, 0x75, 0x63, 0x6b, 0x65, 0x74, 0x18,
	0x02, 0x20, 0x01, 0x28, 0x09, 0x52, 0x0e, 0x61, 0x72, 0x63, 0x68, 0x69, 0x76, 0x61, 0x6c, 0x42,
	0x75, 0x63, 0x6b, 0x65, 0x74, 0x12, 0x2f, 0x0a, 0x0c, 0x62, 0x61, 0x64, 0x5f, 0x62, 0x69, 0x6e,
	0x61, 0x72, 0x69, 0x65, 0x73, 0x18, 0x03, 0x20, 0x01, 0x28, 0x0b, 0x32, 0x0c, 0x2e, 0x42, 0x61,
	0x64, 0x42, 0x69, 0x6e, 0x61, 0x72, 0x69, 0x65, 0x73, 0x52, 0x0b, 0x62, 0x61, 0x64, 0x42, 0x69,
	0x6e, 0x61, 0x72, 0x69, 0x65, 0x73, 0x12, 0x44, 0x0a, 0x16, 0x68, 0x69, 0x73, 0x74, 0x6f, 0x72,
	0x79, 0x5f, 0x61, 0x72, 0x63, 0x68, 0x69, 0x76, 0x61, 0x6c, 0x5f, 0x73, 0x74, 0x61, 0x74, 0x65,
	0x18, 0x04, 0x20, 0x01, 0x28, 0x0e, 0x32, 0x0e, 0x2e, 0x41, 0x72, 0x63, 0x68, 0x69, 0x76, 0x61,
	0x6c, 0x53, 0x74, 0x61, 0x74, 0x65, 0x52, 0x14, 0x68, 0x69, 0x73, 0x74, 0x6f, 0x72, 0x79, 0x41,
	0x72, 0x63, 0x68, 0x69, 0x76, 0x61, 0x6c, 0x53, 0x74, 0x61, 0x74, 0x65, 0x12, 0x30, 0x0a, 0x14,
	0x68, 0x69, 0x73, 0x74, 0x6f, 0x72, 0x79, 0x5f, 0x61, 0x72, 0x63, 0x68, 0x69, 0x76, 0x61, 0x6c,
	0x5f, 0x75, 0x72, 0x69, 0x18, 0x05, 0x20, 0x01, 0x28, 0x09, 0x52, 0x12, 0x68, 0x69, 0x73, 0x74,
	0x6f, 0x72, 0x79, 0x41, 0x72, 0x63, 0x68, 0x69, 0x76, 0x61, 0x6c, 0x55, 0x72, 0x69, 0x12, 0x4a,
	0x0a, 0x19, 0x76, 0x69, 0x73, 0x69, 0x62, 0x69, 0x6c, 0x69, 0x74, 0x79, 0x5f, 0x61, 0x72, 0x63,
	0x68, 0x69, 0x76, 0x61, 0x6c, 0x5f, 0x73, 0x74, 0x61, 0x74, 0x65, 0x18, 0x06, 0x20, 0x01, 0x28,
	0x0e, 0x32, 0x0e, 0x2e, 0x41, 0x72, 0x63, 0x68, 0x69, 0x76, 0x61, 0x6c, 0x53, 0x74, 0x61, 0x74,
	0x65, 0x52, 0x17, 0x76, 0x69, 0x73, 0x69, 0x62, 0x69, 0x6c, 0x69, 0x74, 0x79, 0x41, 0x72, 0x63,
	0x68, 0x69, 0x76, 0x61, 0x6c, 0x53, 0x74, 0x61, 0x74, 0x65, 0x12, 0x36, 0x0a, 0x17, 0x76, 0x69,
	0x73, 0x69, 0x62, 0x69, 0x6c, 0x69, 0x74, 0x79, 0x5f, 0x61, 0x72, 0x63, 0x68, 0x69, 0x76, 0x61,
	0x6c, 0x5f, 0x75, 0x72, 0x69, 0x18, 0x07, 0x20, 0x01, 0x28, 0x09, 0x52, 0x15, 0x76, 0x69, 0x73,
	0x69, 0x62, 0x69, 0x6c, 0x69, 0x74, 0x79, 0x41, 0x72, 0x63, 0x68, 0x69, 0x76, 0x61, 0x6c, 0x55,
	0x72, 0x69, 0x22, 0x92, 0x01, 0x0a, 0x0b, 0x42, 0x61, 0x64, 0x42, 0x69, 0x6e, 0x61, 0x72, 0x69,
	0x65, 0x73, 0x12, 0x36, 0x0a, 0x08, 0x62, 0x69, 0x6e, 0x61, 0x72, 0x69, 0x65, 0x73, 0x18, 0x01,
	0x20, 0x03, 0x28, 0x0b, 0x32, 0x1a, 0x2e, 0x42, 0x61, 0x64, 0x42, 0x69, 0x6e, 0x61, 0x72, 0x69,
	0x65, 0x73, 0x2e, 0x42, 0x69, 0x6e, 0x61, 0x72, 0x69, 0x65, 0x73, 0x45, 0x6e, 0x74, 0x72, 0x79,
	0x52, 0x08, 0x62, 0x69, 0x6e, 0x61, 0x72, 0x69, 0x65, 0x73, 0x1a, 0x4b, 0x0a, 0x0d, 0x42, 0x69,
	0x6e, 0x61, 0x72, 0x69, 0x65, 0x73, 0x45, 0x6e, 0x74, 0x72, 0x79, 0x12, 0x10, 0x0a, 0x03, 0x6b,
	0x65, 0x79, 0x18, 0x01, 0x20, 0x01, 0x28, 0x09, 0x52, 0x03, 0x6b, 0x65, 0x79, 0x12, 0x24, 0x0a,
	0x05, 0x76, 0x61, 0x6c, 0x75, 0x65, 0x18, 0x02, 0x20, 0x01, 0x28, 0x0b, 0x32, 0x0e, 0x2e, 0x42,
	0x61, 0x64, 0x42, 0x69, 0x6e, 0x61, 0x72, 0x79, 0x49, 0x6e, 0x66, 0x6f, 0x52, 0x05, 0x76, 0x61,
	0x6c, 0x75, 0x65, 0x3a, 0x02, 0x38, 0x01, 0x22, 0x86, 0x01, 0x0a, 0x0d, 0x42, 0x61, 0x64, 0x42,
	0x69, 0x6e, 0x61, 0x72, 0x79, 0x49, 0x6e, 0x66, 0x6f, 0x12, 0x16, 0x0a, 0x06, 0x72, 0x65, 0x61,
	0x73, 0x6f, 0x6e, 0x18, 0x01, 0x20, 0x01, 0x28, 0x09, 0x52, 0x06, 0x72, 0x65, 0x61, 0x73, 0x6f,
	0x6e, 0x12, 0x1a, 0x0a, 0x08, 0x6f, 0x70, 0x65, 0x72, 0x61, 0x74, 0x6f, 0x72, 0x18, 0x02, 0x20,
	0x01, 0x28, 0x09, 0x52, 0x08, 0x6f, 0x70, 0x65, 0x72, 0x61, 0x74, 0x6f, 0x72, 0x12, 0x41, 0x0a,
	0x0b, 0x63, 0x72, 0x65, 0x61, 0x74, 0x65, 0x5f, 0x74, 0x69, 0x6d, 0x65, 0x18, 0x03, 0x20, 0x01,
	0x28, 0x0b, 0x32, 0x1a, 0x2e, 0x67, 0x6f, 0x6f, 0x67, 0x6c, 0x65, 0x2e, 0x70, 0x72, 0x6f, 0x74,
	0x6f, 0x62, 0x75, 0x66, 0x2e, 0x54, 0x69, 0x6d, 0x65, 0x73, 0x74, 0x61, 0x6d, 0x70, 0x42, 0x04,
	0x90, 0xdf, 0x1f, 0x01, 0x52, 0x0a, 0x63, 0x72, 0x65, 0x61, 0x74, 0x65, 0x54, 0x69, 0x6d, 0x65,
	0x2a, 0x68, 0x0a, 0x0d, 0x41, 0x72, 0x63, 0x68, 0x69, 0x76, 0x61, 0x6c, 0x53, 0x74, 0x61, 0x74,
	0x65, 0x12, 0x1e, 0x0a, 0x1a, 0x41, 0x52, 0x43, 0x48, 0x49, 0x56, 0x41, 0x4c, 0x5f, 0x53, 0x54,
	0x41, 0x54, 0x45, 0x5f, 0x55, 0x4e, 0x53, 0x50, 0x45, 0x43, 0x49, 0x46, 0x49, 0x45, 0x44, 0x10,
	0x00, 0x12, 0x1b, 0x0a, 0x17, 0x41, 0x52, 0x43, 0x48, 0x49, 0x56, 0x41, 0x4c, 0x5f, 0x53, 0x54,
	0x41, 0x54, 0x45, 0x5f, 0x44, 0x49, 0x53, 0x41, 0x42, 0x4c, 0x45, 0x44, 0x10, 0x01, 0x12, 0x1a,
	0x0a, 0x16, 0x41, 0x52, 0x43, 0x48, 0x49, 0x56, 0x41, 0x4c, 0x5f, 0x53, 0x54, 0x41, 0x54, 0x45,
	0x5f, 0x45, 0x4e, 0x41, 0x42, 0x4c, 0x45, 0x44, 0x10, 0x02, 0x2a, 0x7f, 0x0a, 0x0b, 0x44, 0x6f,
	0x6d, 0x61, 0x69, 0x6e, 0x53, 0x74, 0x61, 0x74, 0x65, 0x12, 0x1c, 0x0a, 0x18, 0x44, 0x4f, 0x4d,
	0x41, 0x49, 0x4e, 0x5f, 0x53, 0x54, 0x41, 0x54, 0x45, 0x5f, 0x55, 0x4e, 0x53, 0x50, 0x45, 0x43,
	0x49, 0x46, 0x49, 0x45, 0x44, 0x10, 0x00, 0x12, 0x1b, 0x0a, 0x17, 0x44, 0x4f, 0x4d, 0x41, 0x49,
	0x4e, 0x5f, 0x53, 0x54, 0x41, 0x54, 0x45, 0x5f, 0x52, 0x45, 0x47, 0x49, 0x53, 0x54, 0x45, 0x52,
	0x45, 0x44, 0x10, 0x01, 0x12, 0x1b, 0x0a, 0x17, 0x44, 0x4f, 0x4d, 0x41, 0x49, 0x4e, 0x5f, 0x53,
	0x54, 0x41, 0x54, 0x45, 0x5f, 0x44, 0x45, 0x50, 0x52, 0x45, 0x43, 0x41, 0x54, 0x45, 0x44, 0x10,
	0x02, 0x12, 0x18, 0x0a, 0x14, 0x44, 0x4f, 0x4d, 0x41, 0x49, 0x4e, 0x5f, 0x53, 0x54, 0x41, 0x54,
	0x45, 0x5f, 0x44, 0x45, 0x4c, 0x45, 0x54, 0x45, 0x44, 0x10, 0x03, 0x42, 0x0c, 0x5a, 0x0a, 0x2e,
	0x67, 0x65, 0x6e, 0x2f, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x62, 0x06, 0x70, 0x72, 0x6f, 0x74, 0x6f,
	0x33,
}

var (
	file_message_proto_rawDescOnce sync.Once
	file_message_proto_rawDescData = file_message_proto_rawDesc
)

func file_message_proto_rawDescGZIP() []byte {
	file_message_proto_rawDescOnce.Do(func() {
		file_message_proto_rawDescData = protoimpl.X.CompressGZIP(file_message_proto_rawDescData)
	})
	return file_message_proto_rawDescData
}

var file_message_proto_enumTypes = make([]protoimpl.EnumInfo, 2)
var file_message_proto_msgTypes = make([]protoimpl.MessageInfo, 8)
var file_message_proto_goTypes = []interface{}{
	(ArchivalState)(0),              // 0: ArchivalState
	(DomainState)(0),                // 1: DomainState
	(*DomainDetail)(nil),            // 2: DomainDetail
	(*DomainInfo)(nil),              // 3: DomainInfo
	(*DomainReplicationConfig)(nil), // 4: DomainReplicationConfig
	(*DomainConfig)(nil),            // 5: DomainConfig
	(*BadBinaries)(nil),             // 6: BadBinaries
	(*BadBinaryInfo)(nil),           // 7: BadBinaryInfo
	nil,                             // 8: DomainInfo.DataEntry
	nil,                             // 9: BadBinaries.BinariesEntry
	(*timestamp.Timestamp)(nil),     // 10: google.protobuf.Timestamp
	(*duration.Duration)(nil),       // 11: google.protobuf.Duration
}
var file_message_proto_depIdxs = []int32{
	3,  // 0: DomainDetail.info:type_name -> DomainInfo
	5,  // 1: DomainDetail.config:type_name -> DomainConfig
	4,  // 2: DomainDetail.replication_config:type_name -> DomainReplicationConfig
	10, // 3: DomainDetail.failover_end_time:type_name -> google.protobuf.Timestamp
	1,  // 4: DomainInfo.state:type_name -> DomainState
	8,  // 5: DomainInfo.data:type_name -> DomainInfo.DataEntry
	11, // 6: DomainConfig.retention:type_name -> google.protobuf.Duration
	6,  // 7: DomainConfig.bad_binaries:type_name -> BadBinaries
	0,  // 8: DomainConfig.history_archival_state:type_name -> ArchivalState
	0,  // 9: DomainConfig.visibility_archival_state:type_name -> ArchivalState
	9,  // 10: BadBinaries.binaries:type_name -> BadBinaries.BinariesEntry
	10, // 11: BadBinaryInfo.create_time:type_name -> google.protobuf.Timestamp
	7,  // 12: BadBinaries.BinariesEntry.value:type_name -> BadBinaryInfo
	13, // [13:13] is the sub-list for method output_type
	13, // [13:13] is the sub-list for method input_type
	13, // [13:13] is the sub-list for extension type_name
	13, // [13:13] is the sub-list for extension extendee
	0,  // [0:13] is the sub-list for field type_name
}

func init() { file_message_proto_init() }
func file_message_proto_init() {
	if File_message_proto != nil {
		return
	}
	file_gogo_proto_init()
	if !protoimpl.UnsafeEnabled {
		file_message_proto_msgTypes[0].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*DomainDetail); i {
			case 0:
				return &v.state
			case 1:
				return &v.sizeCache
			case 2:
				return &v.unknownFields
			default:
				return nil
			}
		}
		file_message_proto_msgTypes[1].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*DomainInfo); i {
			case 0:
				return &v.state
			case 1:
				return &v.sizeCache
			case 2:
				return &v.unknownFields
			default:
				return nil
			}
		}
		file_message_proto_msgTypes[2].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*DomainReplicationConfig); i {
			case 0:
				return &v.state
			case 1:
				return &v.sizeCache
			case 2:
				return &v.unknownFields
			default:
				return nil
			}
		}
		file_message_proto_msgTypes[3].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*DomainConfig); i {
			case 0:
				return &v.state
			case 1:
				return &v.sizeCache
			case 2:
				return &v.unknownFields
			default:
				return nil
			}
		}
		file_message_proto_msgTypes[4].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*BadBinaries); i {
			case 0:
				return &v.state
			case 1:
				return &v.sizeCache
			case 2:
				return &v.unknownFields
			default:
				return nil
			}
		}
		file_message_proto_msgTypes[5].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*BadBinaryInfo); i {
			case 0:
				return &v.state
			case 1:
				return &v.sizeCache
			case 2:
				return &v.unknownFields
			default:
				return nil
			}
		}
	}
	type x struct{}
	out := protoimpl.TypeBuilder{
		File: protoimpl.DescBuilder{
			GoPackagePath: reflect.TypeOf(x{}).PkgPath(),
			RawDescriptor: file_message_proto_rawDesc,
			NumEnums:      2,
			NumMessages:   8,
			NumExtensions: 0,
			NumServices:   0,
		},
		GoTypes:           file_message_proto_goTypes,
		DependencyIndexes: file_message_proto_depIdxs,
		EnumInfos:         file_message_proto_enumTypes,
		MessageInfos:      file_message_proto_msgTypes,
	}.Build()
	File_message_proto = out.File
	file_message_proto_rawDesc = nil
	file_message_proto_goTypes = nil
	file_message_proto_depIdxs = nil
}
