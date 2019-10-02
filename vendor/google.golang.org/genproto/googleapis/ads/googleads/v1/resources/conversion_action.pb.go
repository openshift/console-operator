// Code generated by protoc-gen-go. DO NOT EDIT.
// source: google/ads/googleads/v1/resources/conversion_action.proto

package resources // import "google.golang.org/genproto/googleapis/ads/googleads/v1/resources"

import proto "github.com/golang/protobuf/proto"
import fmt "fmt"
import math "math"
import wrappers "github.com/golang/protobuf/ptypes/wrappers"
import common "google.golang.org/genproto/googleapis/ads/googleads/v1/common"
import enums "google.golang.org/genproto/googleapis/ads/googleads/v1/enums"
import _ "google.golang.org/genproto/googleapis/api/annotations"

// Reference imports to suppress errors if they are not otherwise used.
var _ = proto.Marshal
var _ = fmt.Errorf
var _ = math.Inf

// This is a compile-time assertion to ensure that this generated file
// is compatible with the proto package it is being compiled against.
// A compilation error at this line likely means your copy of the
// proto package needs to be updated.
const _ = proto.ProtoPackageIsVersion2 // please upgrade the proto package

// A conversion action.
type ConversionAction struct {
	// The resource name of the conversion action.
	// Conversion action resource names have the form:
	//
	// `customers/{customer_id}/conversionActions/{conversion_action_id}`
	ResourceName string `protobuf:"bytes,1,opt,name=resource_name,json=resourceName,proto3" json:"resource_name,omitempty"`
	// The ID of the conversion action.
	Id *wrappers.Int64Value `protobuf:"bytes,2,opt,name=id,proto3" json:"id,omitempty"`
	// The name of the conversion action.
	//
	// This field is required and should not be empty when creating new
	// conversion actions.
	Name *wrappers.StringValue `protobuf:"bytes,3,opt,name=name,proto3" json:"name,omitempty"`
	// The status of this conversion action for conversion event accrual.
	Status enums.ConversionActionStatusEnum_ConversionActionStatus `protobuf:"varint,4,opt,name=status,proto3,enum=google.ads.googleads.v1.enums.ConversionActionStatusEnum_ConversionActionStatus" json:"status,omitempty"`
	// The type of this conversion action.
	Type enums.ConversionActionTypeEnum_ConversionActionType `protobuf:"varint,5,opt,name=type,proto3,enum=google.ads.googleads.v1.enums.ConversionActionTypeEnum_ConversionActionType" json:"type,omitempty"`
	// The category of conversions reported for this conversion action.
	Category enums.ConversionActionCategoryEnum_ConversionActionCategory `protobuf:"varint,6,opt,name=category,proto3,enum=google.ads.googleads.v1.enums.ConversionActionCategoryEnum_ConversionActionCategory" json:"category,omitempty"`
	// The resource name of the conversion action owner customer, or null if this
	// is a system-defined conversion action.
	OwnerCustomer *wrappers.StringValue `protobuf:"bytes,7,opt,name=owner_customer,json=ownerCustomer,proto3" json:"owner_customer,omitempty"`
	// Whether this conversion action should be included in the "conversions"
	// metric.
	IncludeInConversionsMetric *wrappers.BoolValue `protobuf:"bytes,8,opt,name=include_in_conversions_metric,json=includeInConversionsMetric,proto3" json:"include_in_conversions_metric,omitempty"`
	// The maximum number of days that may elapse between an interaction
	// (e.g., a click) and a conversion event.
	ClickThroughLookbackWindowDays *wrappers.Int64Value `protobuf:"bytes,9,opt,name=click_through_lookback_window_days,json=clickThroughLookbackWindowDays,proto3" json:"click_through_lookback_window_days,omitempty"`
	// The maximum number of days which may elapse between an impression and a
	// conversion without an interaction.
	ViewThroughLookbackWindowDays *wrappers.Int64Value `protobuf:"bytes,10,opt,name=view_through_lookback_window_days,json=viewThroughLookbackWindowDays,proto3" json:"view_through_lookback_window_days,omitempty"`
	// Settings related to the value for conversion events associated with this
	// conversion action.
	ValueSettings *ConversionAction_ValueSettings `protobuf:"bytes,11,opt,name=value_settings,json=valueSettings,proto3" json:"value_settings,omitempty"`
	// How to count conversion events for the conversion action.
	CountingType enums.ConversionActionCountingTypeEnum_ConversionActionCountingType `protobuf:"varint,12,opt,name=counting_type,json=countingType,proto3,enum=google.ads.googleads.v1.enums.ConversionActionCountingTypeEnum_ConversionActionCountingType" json:"counting_type,omitempty"`
	// Settings related to this conversion action's attribution model.
	AttributionModelSettings *ConversionAction_AttributionModelSettings `protobuf:"bytes,13,opt,name=attribution_model_settings,json=attributionModelSettings,proto3" json:"attribution_model_settings,omitempty"`
	// The snippets used for tracking conversions.
	TagSnippets []*common.TagSnippet `protobuf:"bytes,14,rep,name=tag_snippets,json=tagSnippets,proto3" json:"tag_snippets,omitempty"`
	// The phone call duration in seconds after which a conversion should be
	// reported for this conversion action.
	//
	// The value must be between 0 and 10000, inclusive.
	PhoneCallDurationSeconds *wrappers.Int64Value `protobuf:"bytes,15,opt,name=phone_call_duration_seconds,json=phoneCallDurationSeconds,proto3" json:"phone_call_duration_seconds,omitempty"`
	// App ID for an app conversion action.
	AppId                *wrappers.StringValue `protobuf:"bytes,16,opt,name=app_id,json=appId,proto3" json:"app_id,omitempty"`
	XXX_NoUnkeyedLiteral struct{}              `json:"-"`
	XXX_unrecognized     []byte                `json:"-"`
	XXX_sizecache        int32                 `json:"-"`
}

func (m *ConversionAction) Reset()         { *m = ConversionAction{} }
func (m *ConversionAction) String() string { return proto.CompactTextString(m) }
func (*ConversionAction) ProtoMessage()    {}
func (*ConversionAction) Descriptor() ([]byte, []int) {
	return fileDescriptor_conversion_action_8a96366ce392711c, []int{0}
}
func (m *ConversionAction) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_ConversionAction.Unmarshal(m, b)
}
func (m *ConversionAction) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_ConversionAction.Marshal(b, m, deterministic)
}
func (dst *ConversionAction) XXX_Merge(src proto.Message) {
	xxx_messageInfo_ConversionAction.Merge(dst, src)
}
func (m *ConversionAction) XXX_Size() int {
	return xxx_messageInfo_ConversionAction.Size(m)
}
func (m *ConversionAction) XXX_DiscardUnknown() {
	xxx_messageInfo_ConversionAction.DiscardUnknown(m)
}

var xxx_messageInfo_ConversionAction proto.InternalMessageInfo

func (m *ConversionAction) GetResourceName() string {
	if m != nil {
		return m.ResourceName
	}
	return ""
}

func (m *ConversionAction) GetId() *wrappers.Int64Value {
	if m != nil {
		return m.Id
	}
	return nil
}

func (m *ConversionAction) GetName() *wrappers.StringValue {
	if m != nil {
		return m.Name
	}
	return nil
}

func (m *ConversionAction) GetStatus() enums.ConversionActionStatusEnum_ConversionActionStatus {
	if m != nil {
		return m.Status
	}
	return enums.ConversionActionStatusEnum_UNSPECIFIED
}

func (m *ConversionAction) GetType() enums.ConversionActionTypeEnum_ConversionActionType {
	if m != nil {
		return m.Type
	}
	return enums.ConversionActionTypeEnum_UNSPECIFIED
}

func (m *ConversionAction) GetCategory() enums.ConversionActionCategoryEnum_ConversionActionCategory {
	if m != nil {
		return m.Category
	}
	return enums.ConversionActionCategoryEnum_UNSPECIFIED
}

func (m *ConversionAction) GetOwnerCustomer() *wrappers.StringValue {
	if m != nil {
		return m.OwnerCustomer
	}
	return nil
}

func (m *ConversionAction) GetIncludeInConversionsMetric() *wrappers.BoolValue {
	if m != nil {
		return m.IncludeInConversionsMetric
	}
	return nil
}

func (m *ConversionAction) GetClickThroughLookbackWindowDays() *wrappers.Int64Value {
	if m != nil {
		return m.ClickThroughLookbackWindowDays
	}
	return nil
}

func (m *ConversionAction) GetViewThroughLookbackWindowDays() *wrappers.Int64Value {
	if m != nil {
		return m.ViewThroughLookbackWindowDays
	}
	return nil
}

func (m *ConversionAction) GetValueSettings() *ConversionAction_ValueSettings {
	if m != nil {
		return m.ValueSettings
	}
	return nil
}

func (m *ConversionAction) GetCountingType() enums.ConversionActionCountingTypeEnum_ConversionActionCountingType {
	if m != nil {
		return m.CountingType
	}
	return enums.ConversionActionCountingTypeEnum_UNSPECIFIED
}

func (m *ConversionAction) GetAttributionModelSettings() *ConversionAction_AttributionModelSettings {
	if m != nil {
		return m.AttributionModelSettings
	}
	return nil
}

func (m *ConversionAction) GetTagSnippets() []*common.TagSnippet {
	if m != nil {
		return m.TagSnippets
	}
	return nil
}

func (m *ConversionAction) GetPhoneCallDurationSeconds() *wrappers.Int64Value {
	if m != nil {
		return m.PhoneCallDurationSeconds
	}
	return nil
}

func (m *ConversionAction) GetAppId() *wrappers.StringValue {
	if m != nil {
		return m.AppId
	}
	return nil
}

// Settings related to this conversion action's attribution model.
type ConversionAction_AttributionModelSettings struct {
	// The attribution model type of this conversion action.
	AttributionModel enums.AttributionModelEnum_AttributionModel `protobuf:"varint,1,opt,name=attribution_model,json=attributionModel,proto3,enum=google.ads.googleads.v1.enums.AttributionModelEnum_AttributionModel" json:"attribution_model,omitempty"`
	// The status of the data-driven attribution model for the conversion
	// action.
	DataDrivenModelStatus enums.DataDrivenModelStatusEnum_DataDrivenModelStatus `protobuf:"varint,2,opt,name=data_driven_model_status,json=dataDrivenModelStatus,proto3,enum=google.ads.googleads.v1.enums.DataDrivenModelStatusEnum_DataDrivenModelStatus" json:"data_driven_model_status,omitempty"`
	XXX_NoUnkeyedLiteral  struct{}                                              `json:"-"`
	XXX_unrecognized      []byte                                                `json:"-"`
	XXX_sizecache         int32                                                 `json:"-"`
}

func (m *ConversionAction_AttributionModelSettings) Reset() {
	*m = ConversionAction_AttributionModelSettings{}
}
func (m *ConversionAction_AttributionModelSettings) String() string { return proto.CompactTextString(m) }
func (*ConversionAction_AttributionModelSettings) ProtoMessage()    {}
func (*ConversionAction_AttributionModelSettings) Descriptor() ([]byte, []int) {
	return fileDescriptor_conversion_action_8a96366ce392711c, []int{0, 0}
}
func (m *ConversionAction_AttributionModelSettings) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_ConversionAction_AttributionModelSettings.Unmarshal(m, b)
}
func (m *ConversionAction_AttributionModelSettings) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_ConversionAction_AttributionModelSettings.Marshal(b, m, deterministic)
}
func (dst *ConversionAction_AttributionModelSettings) XXX_Merge(src proto.Message) {
	xxx_messageInfo_ConversionAction_AttributionModelSettings.Merge(dst, src)
}
func (m *ConversionAction_AttributionModelSettings) XXX_Size() int {
	return xxx_messageInfo_ConversionAction_AttributionModelSettings.Size(m)
}
func (m *ConversionAction_AttributionModelSettings) XXX_DiscardUnknown() {
	xxx_messageInfo_ConversionAction_AttributionModelSettings.DiscardUnknown(m)
}

var xxx_messageInfo_ConversionAction_AttributionModelSettings proto.InternalMessageInfo

func (m *ConversionAction_AttributionModelSettings) GetAttributionModel() enums.AttributionModelEnum_AttributionModel {
	if m != nil {
		return m.AttributionModel
	}
	return enums.AttributionModelEnum_UNSPECIFIED
}

func (m *ConversionAction_AttributionModelSettings) GetDataDrivenModelStatus() enums.DataDrivenModelStatusEnum_DataDrivenModelStatus {
	if m != nil {
		return m.DataDrivenModelStatus
	}
	return enums.DataDrivenModelStatusEnum_UNSPECIFIED
}

// Settings related to the value for conversion events associated with this
// conversion action.
type ConversionAction_ValueSettings struct {
	// The value to use when conversion events for this conversion action are
	// sent with an invalid, disallowed or missing value, or when
	// this conversion action is configured to always use the default value.
	DefaultValue *wrappers.DoubleValue `protobuf:"bytes,1,opt,name=default_value,json=defaultValue,proto3" json:"default_value,omitempty"`
	// The currency code to use when conversion events for this conversion
	// action are sent with an invalid or missing currency code, or when this
	// conversion action is configured to always use the default value.
	DefaultCurrencyCode *wrappers.StringValue `protobuf:"bytes,2,opt,name=default_currency_code,json=defaultCurrencyCode,proto3" json:"default_currency_code,omitempty"`
	// Controls whether the default value and default currency code are used in
	// place of the value and currency code specified in conversion events for
	// this conversion action.
	AlwaysUseDefaultValue *wrappers.BoolValue `protobuf:"bytes,3,opt,name=always_use_default_value,json=alwaysUseDefaultValue,proto3" json:"always_use_default_value,omitempty"`
	XXX_NoUnkeyedLiteral  struct{}            `json:"-"`
	XXX_unrecognized      []byte              `json:"-"`
	XXX_sizecache         int32               `json:"-"`
}

func (m *ConversionAction_ValueSettings) Reset()         { *m = ConversionAction_ValueSettings{} }
func (m *ConversionAction_ValueSettings) String() string { return proto.CompactTextString(m) }
func (*ConversionAction_ValueSettings) ProtoMessage()    {}
func (*ConversionAction_ValueSettings) Descriptor() ([]byte, []int) {
	return fileDescriptor_conversion_action_8a96366ce392711c, []int{0, 1}
}
func (m *ConversionAction_ValueSettings) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_ConversionAction_ValueSettings.Unmarshal(m, b)
}
func (m *ConversionAction_ValueSettings) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_ConversionAction_ValueSettings.Marshal(b, m, deterministic)
}
func (dst *ConversionAction_ValueSettings) XXX_Merge(src proto.Message) {
	xxx_messageInfo_ConversionAction_ValueSettings.Merge(dst, src)
}
func (m *ConversionAction_ValueSettings) XXX_Size() int {
	return xxx_messageInfo_ConversionAction_ValueSettings.Size(m)
}
func (m *ConversionAction_ValueSettings) XXX_DiscardUnknown() {
	xxx_messageInfo_ConversionAction_ValueSettings.DiscardUnknown(m)
}

var xxx_messageInfo_ConversionAction_ValueSettings proto.InternalMessageInfo

func (m *ConversionAction_ValueSettings) GetDefaultValue() *wrappers.DoubleValue {
	if m != nil {
		return m.DefaultValue
	}
	return nil
}

func (m *ConversionAction_ValueSettings) GetDefaultCurrencyCode() *wrappers.StringValue {
	if m != nil {
		return m.DefaultCurrencyCode
	}
	return nil
}

func (m *ConversionAction_ValueSettings) GetAlwaysUseDefaultValue() *wrappers.BoolValue {
	if m != nil {
		return m.AlwaysUseDefaultValue
	}
	return nil
}

func init() {
	proto.RegisterType((*ConversionAction)(nil), "google.ads.googleads.v1.resources.ConversionAction")
	proto.RegisterType((*ConversionAction_AttributionModelSettings)(nil), "google.ads.googleads.v1.resources.ConversionAction.AttributionModelSettings")
	proto.RegisterType((*ConversionAction_ValueSettings)(nil), "google.ads.googleads.v1.resources.ConversionAction.ValueSettings")
}

func init() {
	proto.RegisterFile("google/ads/googleads/v1/resources/conversion_action.proto", fileDescriptor_conversion_action_8a96366ce392711c)
}

var fileDescriptor_conversion_action_8a96366ce392711c = []byte{
	// 971 bytes of a gzipped FileDescriptorProto
	0x1f, 0x8b, 0x08, 0x00, 0x00, 0x00, 0x00, 0x00, 0x02, 0xff, 0x9c, 0x56, 0xdd, 0x6e, 0xe3, 0x44,
	0x14, 0x56, 0xd2, 0x6e, 0xd9, 0x9d, 0x26, 0xd9, 0x32, 0xa8, 0x92, 0x95, 0xfd, 0x51, 0xbb, 0x68,
	0xa5, 0x0a, 0x24, 0x67, 0x93, 0x05, 0x24, 0x02, 0x42, 0x72, 0x13, 0xb4, 0x2a, 0x6a, 0x57, 0x95,
	0x53, 0x8a, 0xb4, 0x0a, 0x1a, 0x26, 0x9e, 0xa9, 0x63, 0xd5, 0x9e, 0x31, 0x33, 0xe3, 0x44, 0xb9,
	0x84, 0x1b, 0x24, 0x5e, 0x80, 0x7b, 0x2e, 0x79, 0x03, 0x5e, 0x81, 0x47, 0xe1, 0x0d, 0xb8, 0x43,
	0x1e, 0x8f, 0xdd, 0x34, 0x89, 0x9b, 0x66, 0xef, 0x66, 0x7c, 0xce, 0xf7, 0x7d, 0xe7, 0x9c, 0x39,
	0x73, 0xc6, 0xe0, 0x4b, 0x9f, 0x73, 0x3f, 0xa4, 0x2d, 0x4c, 0x64, 0x2b, 0x5b, 0xa6, 0xab, 0x49,
	0xbb, 0x25, 0xa8, 0xe4, 0x89, 0xf0, 0xa8, 0x6c, 0x79, 0x9c, 0x4d, 0xa8, 0x90, 0x01, 0x67, 0x08,
	0x7b, 0x2a, 0xe0, 0xcc, 0x8e, 0x05, 0x57, 0x1c, 0x1e, 0x66, 0xfe, 0x36, 0x26, 0xd2, 0x2e, 0xa0,
	0xf6, 0xa4, 0x6d, 0x17, 0xd0, 0xe6, 0xab, 0x32, 0x76, 0x8f, 0x47, 0x11, 0x67, 0x2d, 0x85, 0x7d,
	0x24, 0x59, 0x10, 0xc7, 0x54, 0x65, 0xa4, 0xcd, 0xcf, 0xcb, 0x10, 0x94, 0x25, 0x91, 0x6c, 0x61,
	0xa5, 0x44, 0x30, 0x4a, 0xd2, 0x28, 0x50, 0xc4, 0x09, 0x0d, 0x0d, 0xec, 0x9b, 0xbb, 0x61, 0x4b,
	0x29, 0x20, 0x0f, 0x2b, 0xea, 0x73, 0x31, 0x33, 0xf8, 0xde, 0xc6, 0x78, 0x9e, 0x30, 0x15, 0x30,
	0x1f, 0xa9, 0x59, 0x4c, 0x0d, 0xc9, 0xd7, 0x9b, 0x92, 0x48, 0x85, 0x55, 0x22, 0x0d, 0xba, 0xbb,
	0x29, 0xfa, 0xfe, 0xca, 0x04, 0x2b, 0x8c, 0x88, 0x08, 0x26, 0xd4, 0x54, 0xed, 0xb6, 0xf2, 0x73,
	0x83, 0xd6, 0xbb, 0x51, 0x72, 0xd5, 0x9a, 0x0a, 0x1c, 0xc7, 0x54, 0xe4, 0xf6, 0xa7, 0x39, 0x7b,
	0x1c, 0xb4, 0x30, 0x63, 0x5c, 0xe1, 0x54, 0xdf, 0x58, 0x5f, 0xfc, 0xfd, 0x18, 0xec, 0xf5, 0x8a,
	0xe0, 0x1c, 0x1d, 0x1b, 0xfc, 0x18, 0xd4, 0xf3, 0x2e, 0x40, 0x0c, 0x47, 0xd4, 0xaa, 0x1c, 0x54,
	0x8e, 0x1e, 0xb9, 0xb5, 0xfc, 0xe3, 0x5b, 0x1c, 0x51, 0xf8, 0x29, 0xa8, 0x06, 0xc4, 0xaa, 0x1e,
	0x54, 0x8e, 0x76, 0x3b, 0x4f, 0x4c, 0x0b, 0xd9, 0x79, 0x10, 0xf6, 0x09, 0x53, 0x5f, 0x7c, 0x76,
	0x89, 0xc3, 0x84, 0xba, 0xd5, 0x80, 0xc0, 0x57, 0x60, 0x5b, 0x13, 0x6d, 0x69, 0xf7, 0xa7, 0x4b,
	0xee, 0x03, 0x25, 0x02, 0xe6, 0x67, 0xfe, 0xda, 0x13, 0x8e, 0xc1, 0x4e, 0x96, 0xa6, 0xb5, 0x7d,
	0x50, 0x39, 0x6a, 0x74, 0xce, 0xed, 0xb2, 0x86, 0xd5, 0x55, 0xb2, 0x17, 0x93, 0x18, 0x68, 0xf0,
	0xb7, 0x2c, 0x89, 0x4a, 0x4c, 0xae, 0xe1, 0x87, 0x3f, 0x81, 0xed, 0xf4, 0x30, 0xac, 0x07, 0x5a,
	0xe7, 0x74, 0x43, 0x9d, 0x8b, 0x59, 0x4c, 0x57, 0xaa, 0xa4, 0x06, 0x57, 0x33, 0xc3, 0x18, 0x3c,
	0xcc, 0x3b, 0xd6, 0xda, 0xd1, 0x2a, 0x17, 0x1b, 0xaa, 0xf4, 0x0c, 0x7c, 0xa5, 0x52, 0x6e, 0x74,
	0x0b, 0x15, 0xd8, 0x03, 0x0d, 0x3e, 0x65, 0x54, 0x20, 0x2f, 0x91, 0x8a, 0x47, 0x54, 0x58, 0x1f,
	0xdc, 0xa3, 0xf2, 0x75, 0x8d, 0xe9, 0x19, 0x08, 0xfc, 0x11, 0x3c, 0x0b, 0x98, 0x17, 0x26, 0x84,
	0xa2, 0x20, 0xbd, 0x33, 0xb9, 0xaa, 0x44, 0x11, 0x55, 0x22, 0xf0, 0xac, 0x87, 0x9a, 0xb3, 0xb9,
	0xc4, 0x79, 0xcc, 0x79, 0x98, 0x31, 0x36, 0x0d, 0xc1, 0x09, 0xbb, 0x09, 0x5a, 0x9e, 0x69, 0x34,
	0xf4, 0xc1, 0x0b, 0x2f, 0x0c, 0xbc, 0x6b, 0xa4, 0xc6, 0x82, 0x27, 0xfe, 0x18, 0x85, 0x9c, 0x5f,
	0x8f, 0xb0, 0x77, 0x8d, 0xa6, 0x01, 0x23, 0x7c, 0x8a, 0x08, 0x9e, 0x49, 0xeb, 0xd1, 0xfa, 0x06,
	0x7b, 0xae, 0x69, 0x2e, 0x32, 0x96, 0x53, 0x43, 0xf2, 0x83, 0xe6, 0xe8, 0xe3, 0x99, 0x84, 0x14,
	0x1c, 0x4e, 0x02, 0x3a, 0xbd, 0x5b, 0x07, 0xac, 0xd7, 0x79, 0x96, 0xb2, 0x94, 0xcb, 0x8c, 0x41,
	0x63, 0x92, 0xfa, 0x21, 0x49, 0x55, 0x3a, 0x5c, 0xa4, 0xb5, 0xab, 0x39, 0x1d, 0x7b, 0xed, 0xa8,
	0x5d, 0x3a, 0x52, 0x5b, 0x2b, 0x0e, 0x0c, 0x91, 0x5b, 0x9f, 0xcc, 0x6f, 0xe1, 0x2f, 0x15, 0x50,
	0xbf, 0x35, 0xc2, 0xac, 0x9a, 0xee, 0xaa, 0xe1, 0xa6, 0x5d, 0x65, 0x38, 0x4a, 0x7b, 0x78, 0xde,
	0xc1, 0xad, 0x79, 0x73, 0x3b, 0xf8, 0x7b, 0x05, 0x34, 0x97, 0xe6, 0xf9, 0x4d, 0xea, 0x75, 0x9d,
	0xfa, 0xe9, 0xfb, 0xa4, 0xee, 0xdc, 0xb0, 0x9e, 0xa5, 0xa4, 0x45, 0x15, 0x2c, 0x5c, 0x62, 0x81,
	0x67, 0xa0, 0x36, 0xf7, 0x18, 0x49, 0xab, 0x71, 0xb0, 0x75, 0xb4, 0xdb, 0xf9, 0xa4, 0x54, 0x3d,
	0x7b, 0xc0, 0xec, 0x0b, 0xec, 0x0f, 0x32, 0x88, 0xbb, 0xab, 0x8a, 0xb5, 0x84, 0xef, 0xc0, 0x93,
	0x78, 0xcc, 0x19, 0x45, 0x1e, 0x0e, 0x43, 0x44, 0x12, 0x81, 0xb3, 0x81, 0x4f, 0x3d, 0xce, 0x88,
	0xb4, 0x1e, 0xaf, 0x6f, 0x15, 0x4b, 0xe3, 0x7b, 0x38, 0x0c, 0xfb, 0x06, 0x3d, 0xc8, 0xc0, 0xf0,
	0x35, 0xd8, 0xc1, 0x71, 0x8c, 0x02, 0x62, 0xed, 0xdd, 0xe3, 0x46, 0x3e, 0xc0, 0x71, 0x7c, 0x42,
	0x9a, 0x7f, 0x54, 0x81, 0x55, 0x56, 0x16, 0xf8, 0x33, 0xf8, 0x70, 0xe9, 0x20, 0xf4, 0xc4, 0x6e,
	0x74, 0xfa, 0x6b, 0x1a, 0x62, 0x91, 0x53, 0x37, 0xc1, 0xe2, 0x47, 0x77, 0x6f, 0xb1, 0xee, 0xf0,
	0xb7, 0x0a, 0xb0, 0xca, 0x9e, 0x25, 0xfd, 0x24, 0x34, 0x3a, 0x6f, 0xd7, 0x48, 0xf7, 0xb1, 0xc2,
	0x7d, 0x8d, 0xce, 0xb2, 0xb9, 0x19, 0xd7, 0x2b, 0x2d, 0xee, 0x3e, 0x59, 0xf5, 0xb9, 0xf9, 0x5f,
	0x05, 0xd4, 0x6f, 0xdd, 0x15, 0xe8, 0x80, 0x3a, 0xa1, 0x57, 0x38, 0x09, 0x15, 0xd2, 0xb7, 0x46,
	0x97, 0x62, 0x55, 0x9d, 0xfb, 0x3c, 0x19, 0x85, 0x34, 0xab, 0x73, 0xcd, 0x40, 0xf4, 0x0e, 0x9e,
	0x83, 0xfd, 0x9c, 0xc2, 0x4b, 0x84, 0xa0, 0xcc, 0x9b, 0x21, 0x8f, 0x13, 0x6a, 0x5e, 0xbb, 0xbb,
	0x8f, 0xec, 0x23, 0x03, 0xed, 0x19, 0x64, 0x8f, 0x13, 0x0a, 0x07, 0xc0, 0xc2, 0xe1, 0x14, 0xcf,
	0x24, 0x4a, 0x24, 0x45, 0xb7, 0xe3, 0xdb, 0x5a, 0x3b, 0x45, 0xf7, 0x33, 0xec, 0xf7, 0x92, 0xf6,
	0xe7, 0xc2, 0x3c, 0xfe, 0xb5, 0x0a, 0x5e, 0x7a, 0x3c, 0x5a, 0x7f, 0xc7, 0x8e, 0xf7, 0x17, 0x2f,
	0xd9, 0x79, 0x2a, 0x72, 0x5e, 0x79, 0xf7, 0x9d, 0xc1, 0xfa, 0x3c, 0xc4, 0xcc, 0xb7, 0xb9, 0xf0,
	0x5b, 0x3e, 0x65, 0x3a, 0x84, 0xfc, 0x57, 0x24, 0x0e, 0xe4, 0x1d, 0xff, 0x97, 0x5f, 0x15, 0xab,
	0x3f, 0xab, 0x5b, 0x6f, 0x1c, 0xe7, 0xaf, 0xea, 0xe1, 0x9b, 0x8c, 0xd2, 0x21, 0xd2, 0xce, 0x96,
	0xe9, 0xea, 0xb2, 0x6d, 0xbb, 0xb9, 0xe7, 0x3f, 0xb9, 0xcf, 0xd0, 0x21, 0x72, 0x58, 0xf8, 0x0c,
	0x2f, 0xdb, 0xc3, 0xc2, 0xe7, 0xdf, 0xea, 0xcb, 0xcc, 0xd0, 0xed, 0x3a, 0x44, 0x76, 0xbb, 0x85,
	0x57, 0xb7, 0x7b, 0xd9, 0xee, 0x76, 0x0b, 0xbf, 0xd1, 0x8e, 0x0e, 0xf6, 0xf5, 0xff, 0x01, 0x00,
	0x00, 0xff, 0xff, 0xc5, 0x8f, 0x6f, 0xb6, 0x0b, 0x0b, 0x00, 0x00,
}
