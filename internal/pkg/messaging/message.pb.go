// Code generated by protoc-gen-go. DO NOT EDIT.
// versions:
// 	protoc-gen-go v1.36.6
// 	protoc        v5.29.3
// source: internal/pkg/messaging/message.proto

package messaging

import (
	protoreflect "google.golang.org/protobuf/reflect/protoreflect"
	protoimpl "google.golang.org/protobuf/runtime/protoimpl"
	reflect "reflect"
	sync "sync"
	unsafe "unsafe"
)

const (
	// Verify that this generated code is sufficiently up-to-date.
	_ = protoimpl.EnforceVersion(20 - protoimpl.MinVersion)
	// Verify that runtime/protoimpl is sufficiently up-to-date.
	_ = protoimpl.EnforceVersion(protoimpl.MaxVersion - 20)
)

type EventRequest struct {
	state         protoimpl.MessageState `protogen:"open.v1"`
	EventType     string                 `protobuf:"bytes,1,opt,name=eventType,proto3" json:"eventType,omitempty"`
	Topic         string                 `protobuf:"bytes,2,opt,name=topic,proto3" json:"topic,omitempty"`
	ClusterID     string                 `protobuf:"bytes,3,opt,name=clusterID,proto3" json:"clusterID,omitempty"`
	Payload       string                 `protobuf:"bytes,4,opt,name=payload,proto3" json:"payload,omitempty"` // JSON-encoded payload
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *EventRequest) Reset() {
	*x = EventRequest{}
	mi := &file_internal_pkg_messaging_message_proto_msgTypes[0]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *EventRequest) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*EventRequest) ProtoMessage() {}

func (x *EventRequest) ProtoReflect() protoreflect.Message {
	mi := &file_internal_pkg_messaging_message_proto_msgTypes[0]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use EventRequest.ProtoReflect.Descriptor instead.
func (*EventRequest) Descriptor() ([]byte, []int) {
	return file_internal_pkg_messaging_message_proto_rawDescGZIP(), []int{0}
}

func (x *EventRequest) GetEventType() string {
	if x != nil {
		return x.EventType
	}
	return ""
}

func (x *EventRequest) GetTopic() string {
	if x != nil {
		return x.Topic
	}
	return ""
}

func (x *EventRequest) GetClusterID() string {
	if x != nil {
		return x.ClusterID
	}
	return ""
}

func (x *EventRequest) GetPayload() string {
	if x != nil {
		return x.Payload
	}
	return ""
}

type EventResponse struct {
	state         protoimpl.MessageState `protogen:"open.v1"`
	Success       bool                   `protobuf:"varint,1,opt,name=success,proto3" json:"success,omitempty"`
	Message       string                 `protobuf:"bytes,2,opt,name=message,proto3" json:"message,omitempty"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *EventResponse) Reset() {
	*x = EventResponse{}
	mi := &file_internal_pkg_messaging_message_proto_msgTypes[1]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *EventResponse) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*EventResponse) ProtoMessage() {}

func (x *EventResponse) ProtoReflect() protoreflect.Message {
	mi := &file_internal_pkg_messaging_message_proto_msgTypes[1]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use EventResponse.ProtoReflect.Descriptor instead.
func (*EventResponse) Descriptor() ([]byte, []int) {
	return file_internal_pkg_messaging_message_proto_rawDescGZIP(), []int{1}
}

func (x *EventResponse) GetSuccess() bool {
	if x != nil {
		return x.Success
	}
	return false
}

func (x *EventResponse) GetMessage() string {
	if x != nil {
		return x.Message
	}
	return ""
}

type SubscribeRequest struct {
	state         protoimpl.MessageState `protogen:"open.v1"`
	Topic         string                 `protobuf:"bytes,1,opt,name=topic,proto3" json:"topic,omitempty"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *SubscribeRequest) Reset() {
	*x = SubscribeRequest{}
	mi := &file_internal_pkg_messaging_message_proto_msgTypes[2]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *SubscribeRequest) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*SubscribeRequest) ProtoMessage() {}

func (x *SubscribeRequest) ProtoReflect() protoreflect.Message {
	mi := &file_internal_pkg_messaging_message_proto_msgTypes[2]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use SubscribeRequest.ProtoReflect.Descriptor instead.
func (*SubscribeRequest) Descriptor() ([]byte, []int) {
	return file_internal_pkg_messaging_message_proto_rawDescGZIP(), []int{2}
}

func (x *SubscribeRequest) GetTopic() string {
	if x != nil {
		return x.Topic
	}
	return ""
}

type SubscribeResponse struct {
	state         protoimpl.MessageState `protogen:"open.v1"`
	Success       bool                   `protobuf:"varint,1,opt,name=success,proto3" json:"success,omitempty"`
	Message       string                 `protobuf:"bytes,2,opt,name=message,proto3" json:"message,omitempty"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *SubscribeResponse) Reset() {
	*x = SubscribeResponse{}
	mi := &file_internal_pkg_messaging_message_proto_msgTypes[3]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *SubscribeResponse) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*SubscribeResponse) ProtoMessage() {}

func (x *SubscribeResponse) ProtoReflect() protoreflect.Message {
	mi := &file_internal_pkg_messaging_message_proto_msgTypes[3]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use SubscribeResponse.ProtoReflect.Descriptor instead.
func (*SubscribeResponse) Descriptor() ([]byte, []int) {
	return file_internal_pkg_messaging_message_proto_rawDescGZIP(), []int{3}
}

func (x *SubscribeResponse) GetSuccess() bool {
	if x != nil {
		return x.Success
	}
	return false
}

func (x *SubscribeResponse) GetMessage() string {
	if x != nil {
		return x.Message
	}
	return ""
}

var File_internal_pkg_messaging_message_proto protoreflect.FileDescriptor

const file_internal_pkg_messaging_message_proto_rawDesc = "" +
	"\n" +
	"$internal/pkg/messaging/message.proto\x12\tmessaging\"z\n" +
	"\fEventRequest\x12\x1c\n" +
	"\teventType\x18\x01 \x01(\tR\teventType\x12\x14\n" +
	"\x05topic\x18\x02 \x01(\tR\x05topic\x12\x1c\n" +
	"\tclusterID\x18\x03 \x01(\tR\tclusterID\x12\x18\n" +
	"\apayload\x18\x04 \x01(\tR\apayload\"C\n" +
	"\rEventResponse\x12\x18\n" +
	"\asuccess\x18\x01 \x01(\bR\asuccess\x12\x18\n" +
	"\amessage\x18\x02 \x01(\tR\amessage\"(\n" +
	"\x10SubscribeRequest\x12\x14\n" +
	"\x05topic\x18\x01 \x01(\tR\x05topic\"G\n" +
	"\x11SubscribeResponse\x12\x18\n" +
	"\asuccess\x18\x01 \x01(\bR\asuccess\x12\x18\n" +
	"\amessage\x18\x02 \x01(\tR\amessage2\x9e\x01\n" +
	"\fEventService\x12A\n" +
	"\fPublishEvent\x12\x17.messaging.EventRequest\x1a\x18.messaging.EventResponse\x12K\n" +
	"\x0eSubscribeEvent\x12\x1b.messaging.SubscribeRequest\x1a\x1c.messaging.SubscribeResponseB\"Z internal/pkg/messaging;messagingb\x06proto3"

var (
	file_internal_pkg_messaging_message_proto_rawDescOnce sync.Once
	file_internal_pkg_messaging_message_proto_rawDescData []byte
)

func file_internal_pkg_messaging_message_proto_rawDescGZIP() []byte {
	file_internal_pkg_messaging_message_proto_rawDescOnce.Do(func() {
		file_internal_pkg_messaging_message_proto_rawDescData = protoimpl.X.CompressGZIP(unsafe.Slice(unsafe.StringData(file_internal_pkg_messaging_message_proto_rawDesc), len(file_internal_pkg_messaging_message_proto_rawDesc)))
	})
	return file_internal_pkg_messaging_message_proto_rawDescData
}

var file_internal_pkg_messaging_message_proto_msgTypes = make([]protoimpl.MessageInfo, 4)
var file_internal_pkg_messaging_message_proto_goTypes = []any{
	(*EventRequest)(nil),      // 0: messaging.EventRequest
	(*EventResponse)(nil),     // 1: messaging.EventResponse
	(*SubscribeRequest)(nil),  // 2: messaging.SubscribeRequest
	(*SubscribeResponse)(nil), // 3: messaging.SubscribeResponse
}
var file_internal_pkg_messaging_message_proto_depIdxs = []int32{
	0, // 0: messaging.EventService.PublishEvent:input_type -> messaging.EventRequest
	2, // 1: messaging.EventService.SubscribeEvent:input_type -> messaging.SubscribeRequest
	1, // 2: messaging.EventService.PublishEvent:output_type -> messaging.EventResponse
	3, // 3: messaging.EventService.SubscribeEvent:output_type -> messaging.SubscribeResponse
	2, // [2:4] is the sub-list for method output_type
	0, // [0:2] is the sub-list for method input_type
	0, // [0:0] is the sub-list for extension type_name
	0, // [0:0] is the sub-list for extension extendee
	0, // [0:0] is the sub-list for field type_name
}

func init() { file_internal_pkg_messaging_message_proto_init() }
func file_internal_pkg_messaging_message_proto_init() {
	if File_internal_pkg_messaging_message_proto != nil {
		return
	}
	type x struct{}
	out := protoimpl.TypeBuilder{
		File: protoimpl.DescBuilder{
			GoPackagePath: reflect.TypeOf(x{}).PkgPath(),
			RawDescriptor: unsafe.Slice(unsafe.StringData(file_internal_pkg_messaging_message_proto_rawDesc), len(file_internal_pkg_messaging_message_proto_rawDesc)),
			NumEnums:      0,
			NumMessages:   4,
			NumExtensions: 0,
			NumServices:   1,
		},
		GoTypes:           file_internal_pkg_messaging_message_proto_goTypes,
		DependencyIndexes: file_internal_pkg_messaging_message_proto_depIdxs,
		MessageInfos:      file_internal_pkg_messaging_message_proto_msgTypes,
	}.Build()
	File_internal_pkg_messaging_message_proto = out.File
	file_internal_pkg_messaging_message_proto_goTypes = nil
	file_internal_pkg_messaging_message_proto_depIdxs = nil
}
