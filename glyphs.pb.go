// Code generated by protoc-gen-go. DO NOT EDIT.
// source: glyphs.proto

// Protocol Version 1

package main

import (
	fmt "fmt"
	math "math"

	proto "github.com/golang/protobuf/proto"
)

// Reference imports to suppress errors if they are not otherwise used.
var _ = proto.Marshal
var _ = fmt.Errorf
var _ = math.Inf

// This is a compile-time assertion to ensure that this generated file
// is compatible with the proto package it is being compiled against.
// A compilation error at this line likely means your copy of the
// proto package needs to be updated.
const _ = proto.ProtoPackageIsVersion2 // please upgrade the proto package

// Stores a glyph with metrics and optional SDF bitmap information.
type Glyph struct {
	Id *uint32 `protobuf:"varint,1,req,name=id" json:"id,omitempty"`
	// A signed distance field of the glyph with a border of 3 pixels.
	Bitmap []byte `protobuf:"bytes,2,opt,name=bitmap" json:"bitmap,omitempty"`
	// Glyph metrics.
	Width                *uint32  `protobuf:"varint,3,req,name=width" json:"width,omitempty"`
	Height               *uint32  `protobuf:"varint,4,req,name=height" json:"height,omitempty"`
	Left                 *int32   `protobuf:"zigzag32,5,req,name=left" json:"left,omitempty"`
	Top                  *int32   `protobuf:"zigzag32,6,req,name=top" json:"top,omitempty"`
	Advance              *uint32  `protobuf:"varint,7,req,name=advance" json:"advance,omitempty"`
	XXX_NoUnkeyedLiteral struct{} `json:"-"`
	XXX_unrecognized     []byte   `json:"-"`
	XXX_sizecache        int32    `json:"-"`
}

func (m *Glyph) Reset()         { *m = Glyph{} }
func (m *Glyph) String() string { return proto.CompactTextString(m) }
func (*Glyph) ProtoMessage()    {}
func (*Glyph) Descriptor() ([]byte, []int) {
	return fileDescriptor_6bbe9e0d5eab4d4a, []int{0}
}

func (m *Glyph) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_Glyph.Unmarshal(m, b)
}
func (m *Glyph) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_Glyph.Marshal(b, m, deterministic)
}
func (m *Glyph) XXX_Merge(src proto.Message) {
	xxx_messageInfo_Glyph.Merge(m, src)
}
func (m *Glyph) XXX_Size() int {
	return xxx_messageInfo_Glyph.Size(m)
}
func (m *Glyph) XXX_DiscardUnknown() {
	xxx_messageInfo_Glyph.DiscardUnknown(m)
}

var xxx_messageInfo_Glyph proto.InternalMessageInfo

func (m *Glyph) GetId() uint32 {
	if m != nil && m.Id != nil {
		return *m.Id
	}
	return 0
}

func (m *Glyph) GetBitmap() []byte {
	if m != nil {
		return m.Bitmap
	}
	return nil
}

func (m *Glyph) GetWidth() uint32 {
	if m != nil && m.Width != nil {
		return *m.Width
	}
	return 0
}

func (m *Glyph) GetHeight() uint32 {
	if m != nil && m.Height != nil {
		return *m.Height
	}
	return 0
}

func (m *Glyph) GetLeft() int32 {
	if m != nil && m.Left != nil {
		return *m.Left
	}
	return 0
}

func (m *Glyph) GetTop() int32 {
	if m != nil && m.Top != nil {
		return *m.Top
	}
	return 0
}

func (m *Glyph) GetAdvance() uint32 {
	if m != nil && m.Advance != nil {
		return *m.Advance
	}
	return 0
}

// Stores fontstack information and a list of faces.
type Fontstack struct {
	Name                 *string  `protobuf:"bytes,1,req,name=name" json:"name,omitempty"`
	Range                *string  `protobuf:"bytes,2,req,name=range" json:"range,omitempty"`
	Glyphs               []*Glyph `protobuf:"bytes,3,rep,name=glyphs" json:"glyphs,omitempty"`
	XXX_NoUnkeyedLiteral struct{} `json:"-"`
	XXX_unrecognized     []byte   `json:"-"`
	XXX_sizecache        int32    `json:"-"`
}

func (m *Fontstack) Reset()         { *m = Fontstack{} }
func (m *Fontstack) String() string { return proto.CompactTextString(m) }
func (*Fontstack) ProtoMessage()    {}
func (*Fontstack) Descriptor() ([]byte, []int) {
	return fileDescriptor_6bbe9e0d5eab4d4a, []int{1}
}

func (m *Fontstack) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_Fontstack.Unmarshal(m, b)
}
func (m *Fontstack) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_Fontstack.Marshal(b, m, deterministic)
}
func (m *Fontstack) XXX_Merge(src proto.Message) {
	xxx_messageInfo_Fontstack.Merge(m, src)
}
func (m *Fontstack) XXX_Size() int {
	return xxx_messageInfo_Fontstack.Size(m)
}
func (m *Fontstack) XXX_DiscardUnknown() {
	xxx_messageInfo_Fontstack.DiscardUnknown(m)
}

var xxx_messageInfo_Fontstack proto.InternalMessageInfo

func (m *Fontstack) GetName() string {
	if m != nil && m.Name != nil {
		return *m.Name
	}
	return ""
}

func (m *Fontstack) GetRange() string {
	if m != nil && m.Range != nil {
		return *m.Range
	}
	return ""
}

func (m *Fontstack) GetGlyphs() []*Glyph {
	if m != nil {
		return m.Glyphs
	}
	return nil
}

type Glyphs struct {
	Stacks                       []*Fontstack `protobuf:"bytes,1,rep,name=stacks" json:"stacks,omitempty"`
	XXX_NoUnkeyedLiteral         struct{}     `json:"-"`
	proto.XXX_InternalExtensions `json:"-"`
	XXX_unrecognized             []byte `json:"-"`
	XXX_sizecache                int32  `json:"-"`
}

func (m *Glyphs) Reset()         { *m = Glyphs{} }
func (m *Glyphs) String() string { return proto.CompactTextString(m) }
func (*Glyphs) ProtoMessage()    {}
func (*Glyphs) Descriptor() ([]byte, []int) {
	return fileDescriptor_6bbe9e0d5eab4d4a, []int{2}
}

var extRange_Glyphs = []proto.ExtensionRange{
	{Start: 16, End: 8191},
}

func (*Glyphs) ExtensionRangeArray() []proto.ExtensionRange {
	return extRange_Glyphs
}

func (m *Glyphs) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_Glyphs.Unmarshal(m, b)
}
func (m *Glyphs) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_Glyphs.Marshal(b, m, deterministic)
}
func (m *Glyphs) XXX_Merge(src proto.Message) {
	xxx_messageInfo_Glyphs.Merge(m, src)
}
func (m *Glyphs) XXX_Size() int {
	return xxx_messageInfo_Glyphs.Size(m)
}
func (m *Glyphs) XXX_DiscardUnknown() {
	xxx_messageInfo_Glyphs.DiscardUnknown(m)
}

var xxx_messageInfo_Glyphs proto.InternalMessageInfo

func (m *Glyphs) GetStacks() []*Fontstack {
	if m != nil {
		return m.Stacks
	}
	return nil
}

func init() {
	proto.RegisterType((*Glyph)(nil), "main.glyph")
	proto.RegisterType((*Fontstack)(nil), "main.fontstack")
	proto.RegisterType((*Glyphs)(nil), "main.glyphs")
}

func init() { proto.RegisterFile("glyphs.proto", fileDescriptor_6bbe9e0d5eab4d4a) }

var fileDescriptor_6bbe9e0d5eab4d4a = []byte{
	// 243 bytes of a gzipped FileDescriptorProto
	0x1f, 0x8b, 0x08, 0x00, 0x00, 0x00, 0x00, 0x00, 0x02, 0xff, 0x3c, 0x8f, 0xb1, 0x4e, 0xc3, 0x30,
	0x10, 0x86, 0x15, 0x3b, 0x49, 0xe9, 0xb5, 0x40, 0x38, 0x21, 0x74, 0x63, 0x14, 0x06, 0x22, 0x86,
	0x0c, 0x4c, 0x8c, 0x88, 0x89, 0xd9, 0x23, 0x9b, 0x69, 0xdc, 0xc4, 0xa2, 0x71, 0xa2, 0xc6, 0x02,
	0xb1, 0xf1, 0x22, 0xbc, 0x2b, 0xf2, 0xc5, 0xed, 0x76, 0xff, 0x77, 0x9f, 0x7d, 0x77, 0xb0, 0xed,
	0x0e, 0x3f, 0x53, 0x3f, 0x37, 0xd3, 0x71, 0xf4, 0x23, 0xa6, 0x83, 0xb6, 0xae, 0xfa, 0x4b, 0x20,
	0x63, 0x8c, 0x57, 0x20, 0x6c, 0x4b, 0x49, 0x29, 0xea, 0x4b, 0x25, 0x6c, 0x8b, 0x77, 0x90, 0x7f,
	0x58, 0x3f, 0xe8, 0x89, 0x44, 0x99, 0xd4, 0x5b, 0x15, 0x13, 0xde, 0x42, 0xf6, 0x6d, 0x5b, 0xdf,
	0x93, 0x64, 0x75, 0x09, 0xc1, 0xee, 0x8d, 0xed, 0x7a, 0x4f, 0x29, 0xe3, 0x98, 0x10, 0x21, 0x3d,
	0x98, 0xbd, 0xa7, 0xac, 0x14, 0xf5, 0x8d, 0xe2, 0x1a, 0x0b, 0x90, 0x7e, 0x9c, 0x28, 0x67, 0x14,
	0x4a, 0x24, 0x58, 0xe9, 0xf6, 0x4b, 0xbb, 0x9d, 0xa1, 0x15, 0x3f, 0x3f, 0xc5, 0xea, 0x1d, 0xd6,
	0xfb, 0xd1, 0xf9, 0xd9, 0xeb, 0xdd, 0x67, 0xf8, 0xcc, 0xe9, 0xc1, 0xf0, 0x92, 0x6b, 0xc5, 0x75,
	0x58, 0xe7, 0xa8, 0x5d, 0x67, 0x48, 0x30, 0x5c, 0x02, 0xde, 0x43, 0xbe, 0x1c, 0x4b, 0xb2, 0x94,
	0xf5, 0xe6, 0x69, 0xd3, 0x84, 0x6b, 0x1b, 0x66, 0x2a, 0xb6, 0xaa, 0xe7, 0x93, 0x84, 0x0f, 0x90,
	0xf3, 0x84, 0x99, 0x12, 0xd6, 0xaf, 0x17, 0xfd, 0x3c, 0x59, 0xc5, 0xf6, 0x63, 0x76, 0x51, 0x14,
	0xbf, 0x2f, 0xaf, 0xe2, 0x4d, 0xfe, 0x07, 0x00, 0x00, 0xff, 0xff, 0x9a, 0x31, 0x6c, 0x9f, 0x4e,
	0x01, 0x00, 0x00,
}
