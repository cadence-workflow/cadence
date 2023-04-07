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

// Code generated by protoc-gen-yarpc-go. DO NOT EDIT.
// source: uber/cadence/api/v1/tasklist.proto

package apiv1

var yarpcFileDescriptorClosure216fa006947e00a0 = [][]byte{
	// uber/cadence/api/v1/tasklist.proto
	[]byte{
		0x1f, 0x8b, 0x08, 0x00, 0x00, 0x00, 0x00, 0x00, 0x02, 0xff, 0x8c, 0x54, 0xff, 0x6e, 0xdb, 0x36,
		0x10, 0x9e, 0xe2, 0xb4, 0x4b, 0x98, 0x25, 0xd5, 0x88, 0xb5, 0x8d, 0xdd, 0xfd, 0x08, 0x84, 0xa1,
		0x0b, 0x8a, 0x41, 0x42, 0xb2, 0x3f, 0x37, 0x60, 0x70, 0xe2, 0x00, 0x15, 0xec, 0xb8, 0x86, 0xa4,
		0x05, 0xc8, 0x36, 0x80, 0xa3, 0xc4, 0xab, 0x43, 0xe8, 0x07, 0x05, 0x92, 0x72, 0x92, 0x17, 0xd9,
		0xfb, 0xec, 0x89, 0xf6, 0x0a, 0x03, 0x29, 0xd9, 0x73, 0x13, 0x0f, 0xeb, 0x7f, 0xe4, 0x7d, 0xf7,
		0xdd, 0xf1, 0xbe, 0xbb, 0x23, 0xf2, 0x9a, 0x14, 0x64, 0x90, 0x51, 0x06, 0x55, 0x06, 0x01, 0xad,
		0x79, 0xb0, 0x38, 0x09, 0x34, 0x55, 0x79, 0xc1, 0x95, 0xf6, 0x6b, 0x29, 0xb4, 0xc0, 0x03, 0x05,
		0x72, 0x01, 0xd2, 0x37, 0xae, 0x7e, 0xe7, 0xea, 0xd3, 0x9a, 0xfb, 0x8b, 0x93, 0xc1, 0xd7, 0x73,
		0x21, 0xe6, 0x05, 0x04, 0xd6, 0x33, 0x6d, 0xde, 0x07, 0xac, 0x91, 0x54, 0x73, 0x51, 0xb5, 0xdc,
		0xc1, 0x37, 0x0f, 0x71, 0xcd, 0x4b, 0x50, 0x9a, 0x96, 0x75, 0xe7, 0xf0, 0x28, 0xc0, 0xad, 0xa4,
		0x75, 0x0d, 0x52, 0xb5, 0xb8, 0xf7, 0x3b, 0xda, 0x49, 0xa8, 0xca, 0x27, 0x5c, 0x69, 0x8c, 0xd1,
		0x76, 0x45, 0x4b, 0x38, 0x74, 0x8e, 0x9c, 0xe3, 0xdd, 0xc8, 0x9e, 0xf1, 0x4f, 0x68, 0x3b, 0xe7,
		0x15, 0x3b, 0xdc, 0x3a, 0x72, 0x8e, 0x0f, 0x4e, 0x8f, 0xfd, 0xff, 0x7e, 0xab, 0xbf, 0x8c, 0x33,
		0xe6, 0x15, 0x8b, 0x2c, 0xcb, 0xa3, 0xc8, 0x5d, 0x5a, 0x2f, 0x41, 0x53, 0x46, 0x35, 0xc5, 0x97,
		0xe8, 0x8b, 0x92, 0xde, 0x11, 0x23, 0x82, 0x22, 0x35, 0x48, 0xa2, 0x20, 0x13, 0x15, 0xb3, 0x59,
		0xf7, 0x4e, 0xbf, 0xf4, 0xdb, 0x07, 0xfb, 0xcb, 0x07, 0xfb, 0x23, 0xd1, 0xa4, 0x05, 0x5c, 0xd1,
		0xa2, 0x81, 0xe8, 0xf3, 0x92, 0xde, 0x99, 0x80, 0x6a, 0x06, 0x32, 0xb6, 0x34, 0xef, 0x17, 0xd4,
		0x5f, 0xa6, 0x98, 0x51, 0xa9, 0xb9, 0x11, 0x67, 0x95, 0xcb, 0x45, 0xbd, 0x1c, 0xee, 0xbb, 0x82,
		0xcc, 0x11, 0xbf, 0x46, 0xcf, 0xc4, 0x6d, 0x05, 0x92, 0xdc, 0x08, 0xa5, 0x89, 0x2d, 0x77, 0xcb,
		0xa2, 0xfb, 0xd6, 0xfc, 0x56, 0x28, 0x3d, 0xa5, 0x25, 0x78, 0x7f, 0x3b, 0xe8, 0x60, 0x19, 0x37,
		0xd6, 0x54, 0x37, 0x0a, 0x7f, 0x8f, 0x70, 0x4a, 0xb3, 0xbc, 0x10, 0x73, 0x92, 0x89, 0xa6, 0xd2,
		0xe4, 0x86, 0x57, 0xda, 0xc6, 0xee, 0x45, 0x6e, 0x87, 0x9c, 0x1b, 0xe0, 0x2d, 0xaf, 0x34, 0xfe,
		0x0a, 0x21, 0x09, 0x94, 0x91, 0x02, 0x16, 0x50, 0xd8, 0x1c, 0xbd, 0x68, 0xd7, 0x58, 0x26, 0xc6,
		0x80, 0x5f, 0xa1, 0x5d, 0x9a, 0xe5, 0x1d, 0xda, 0xb3, 0xe8, 0x0e, 0xcd, 0xf2, 0x16, 0x7c, 0x8d,
		0x9e, 0x49, 0xaa, 0x61, 0x5d, 0x9d, 0xed, 0x23, 0xe7, 0xd8, 0x89, 0xf6, 0x8d, 0x79, 0x55, 0x3b,
		0x1e, 0xa3, 0x7d, 0x23, 0x23, 0xe1, 0x8c, 0xa4, 0x85, 0xc8, 0xf2, 0xc3, 0x27, 0x56, 0xc3, 0xef,
		0xfe, 0xaf, 0x4b, 0xe1, 0xe8, 0xcc, 0xb8, 0x47, 0x7b, 0x86, 0x1d, 0x32, 0x7b, 0xf1, 0x7e, 0x46,
		0x7b, 0x6b, 0x18, 0xee, 0xa3, 0x1d, 0xa5, 0xa9, 0xd4, 0x84, 0xb3, 0xae, 0xc6, 0x4f, 0xed, 0x3d,
		0x64, 0xf8, 0x39, 0x7a, 0x0a, 0x15, 0x33, 0x40, 0x5b, 0xd6, 0x13, 0xa8, 0x58, 0xc8, 0xbc, 0x3f,
		0x1d, 0x84, 0x66, 0xa2, 0x28, 0x40, 0x86, 0xd5, 0x7b, 0x81, 0x47, 0xc8, 0x2d, 0xa8, 0xd2, 0x84,
		0x66, 0x19, 0x28, 0x45, 0xcc, 0x60, 0x76, 0x3d, 0x1e, 0x3c, 0xea, 0x71, 0xb2, 0x9c, 0xda, 0xe8,
		0xc0, 0x70, 0x86, 0x96, 0x62, 0x8c, 0x78, 0x80, 0x76, 0x38, 0x83, 0x4a, 0x73, 0x7d, 0xdf, 0x35,
		0x6a, 0x75, 0xdf, 0x24, 0x53, 0x6f, 0x83, 0x4c, 0xde, 0x5f, 0x0e, 0xea, 0xc7, 0x9a, 0x67, 0xf9,
		0xfd, 0xc5, 0x1d, 0x64, 0x8d, 0x99, 0x90, 0xa1, 0xd6, 0x92, 0xa7, 0x8d, 0x06, 0x85, 0xa7, 0xc8,
		0xbd, 0x15, 0x32, 0x07, 0x69, 0x47, 0x92, 0x98, 0xc5, 0xec, 0xde, 0xf9, 0xed, 0xc7, 0x4c, 0x7b,
		0x74, 0xd0, 0xb2, 0x57, 0x5b, 0x94, 0xa0, 0xbe, 0xca, 0x6e, 0x80, 0x35, 0x05, 0x10, 0x2d, 0x48,
		0x2b, 0xa2, 0xa9, 0x5e, 0x34, 0xda, 0x96, 0xb0, 0x77, 0xda, 0x7f, 0x3c, 0xe4, 0xdd, 0x5a, 0x47,
		0x2f, 0x96, 0xdc, 0x44, 0xc4, 0x86, 0x99, 0xb4, 0xc4, 0x37, 0x7f, 0xa0, 0xcf, 0xd6, 0xf7, 0x0b,
		0x0f, 0xd0, 0x8b, 0x64, 0x18, 0x8f, 0xc9, 0x24, 0x8c, 0x13, 0x32, 0x0e, 0xa7, 0x23, 0x12, 0x4e,
		0xaf, 0x86, 0x93, 0x70, 0xe4, 0x7e, 0x82, 0xfb, 0xe8, 0xf9, 0x03, 0x6c, 0xfa, 0x2e, 0xba, 0x1c,
		0x4e, 0x5c, 0x67, 0x03, 0x14, 0x27, 0xe1, 0xf9, 0xf8, 0xda, 0xdd, 0x7a, 0xc3, 0xfe, 0xcd, 0x90,
		0xdc, 0xd7, 0xf0, 0x61, 0x86, 0xe4, 0x7a, 0x76, 0xb1, 0x96, 0xe1, 0x15, 0x7a, 0xf9, 0x00, 0x1b,
		0x5d, 0x9c, 0x87, 0x71, 0xf8, 0x6e, 0xea, 0x3a, 0x1b, 0xc0, 0xe1, 0x79, 0x12, 0x5e, 0x85, 0xc9,
		0xb5, 0xbb, 0x75, 0xf6, 0x1b, 0x7a, 0x99, 0x89, 0x72, 0x93, 0xa2, 0x67, 0xfb, 0xab, 0x3d, 0x36,
		0xaa, 0xcc, 0x9c, 0x5f, 0x83, 0x39, 0xd7, 0x37, 0x4d, 0xea, 0x67, 0xa2, 0x0c, 0x3e, 0xf8, 0x47,
		0xfd, 0x39, 0x54, 0xed, 0x8f, 0xd6, 0x7d, 0xa9, 0x3f, 0xd2, 0x9a, 0x2f, 0x4e, 0xd2, 0xa7, 0xd6,
		0xf6, 0xc3, 0x3f, 0x01, 0x00, 0x00, 0xff, 0xff, 0xca, 0xe9, 0x31, 0x9b, 0x76, 0x05, 0x00, 0x00,
	},
	// google/protobuf/duration.proto
	[]byte{
		0x1f, 0x8b, 0x08, 0x00, 0x00, 0x00, 0x00, 0x00, 0x02, 0xff, 0xe2, 0x92, 0x4b, 0xcf, 0xcf, 0x4f,
		0xcf, 0x49, 0xd5, 0x2f, 0x28, 0xca, 0x2f, 0xc9, 0x4f, 0x2a, 0x4d, 0xd3, 0x4f, 0x29, 0x2d, 0x4a,
		0x2c, 0xc9, 0xcc, 0xcf, 0xd3, 0x03, 0x8b, 0x08, 0xf1, 0x43, 0xe4, 0xf5, 0x60, 0xf2, 0x4a, 0x56,
		0x5c, 0x1c, 0x2e, 0x50, 0x25, 0x42, 0x12, 0x5c, 0xec, 0xc5, 0xa9, 0xc9, 0xf9, 0x79, 0x29, 0xc5,
		0x12, 0x8c, 0x0a, 0x8c, 0x1a, 0xcc, 0x41, 0x30, 0xae, 0x90, 0x08, 0x17, 0x6b, 0x5e, 0x62, 0x5e,
		0x7e, 0xb1, 0x04, 0x93, 0x02, 0xa3, 0x06, 0x6b, 0x10, 0x84, 0xe3, 0xd4, 0xcc, 0xc8, 0x25, 0x9c,
		0x9c, 0x9f, 0xab, 0x87, 0x66, 0xa6, 0x13, 0x2f, 0xcc, 0xc4, 0x00, 0x90, 0x48, 0x00, 0x63, 0x94,
		0x21, 0x54, 0x45, 0x7a, 0x7e, 0x4e, 0x62, 0x5e, 0xba, 0x5e, 0x7e, 0x51, 0x3a, 0xc2, 0x81, 0x25,
		0x95, 0x05, 0xa9, 0xc5, 0xfa, 0xd9, 0x79, 0xf9, 0xe5, 0x79, 0x70, 0xc7, 0x16, 0x24, 0xfd, 0x60,
		0x64, 0x5c, 0xc4, 0xc4, 0xec, 0x1e, 0xe0, 0xb4, 0x8a, 0x49, 0xce, 0x1d, 0xa2, 0x39, 0x00, 0xaa,
		0x43, 0x2f, 0x3c, 0x35, 0x27, 0xc7, 0x1b, 0xa4, 0x3e, 0x04, 0xa4, 0x35, 0x89, 0x0d, 0x6c, 0x94,
		0x31, 0x20, 0x00, 0x00, 0xff, 0xff, 0xef, 0x8a, 0xb4, 0xc3, 0xfb, 0x00, 0x00, 0x00,
	},
	// google/protobuf/timestamp.proto
	[]byte{
		0x1f, 0x8b, 0x08, 0x00, 0x00, 0x00, 0x00, 0x00, 0x02, 0xff, 0xe2, 0x92, 0x4f, 0xcf, 0xcf, 0x4f,
		0xcf, 0x49, 0xd5, 0x2f, 0x28, 0xca, 0x2f, 0xc9, 0x4f, 0x2a, 0x4d, 0xd3, 0x2f, 0xc9, 0xcc, 0x4d,
		0x2d, 0x2e, 0x49, 0xcc, 0x2d, 0xd0, 0x03, 0x0b, 0x09, 0xf1, 0x43, 0x14, 0xe8, 0xc1, 0x14, 0x28,
		0x59, 0x73, 0x71, 0x86, 0xc0, 0xd4, 0x08, 0x49, 0x70, 0xb1, 0x17, 0xa7, 0x26, 0xe7, 0xe7, 0xa5,
		0x14, 0x4b, 0x30, 0x2a, 0x30, 0x6a, 0x30, 0x07, 0xc1, 0xb8, 0x42, 0x22, 0x5c, 0xac, 0x79, 0x89,
		0x79, 0xf9, 0xc5, 0x12, 0x4c, 0x0a, 0x8c, 0x1a, 0xac, 0x41, 0x10, 0x8e, 0x53, 0x2b, 0x23, 0x97,
		0x70, 0x72, 0x7e, 0xae, 0x1e, 0x9a, 0xa1, 0x4e, 0x7c, 0x70, 0x23, 0x03, 0x40, 0x42, 0x01, 0x8c,
		0x51, 0x46, 0x50, 0x25, 0xe9, 0xf9, 0x39, 0x89, 0x79, 0xe9, 0x7a, 0xf9, 0x45, 0xe9, 0x48, 0x6e,
		0xac, 0x2c, 0x48, 0x2d, 0xd6, 0xcf, 0xce, 0xcb, 0x2f, 0xcf, 0x43, 0xb8, 0xb7, 0x20, 0xe9, 0x07,
		0x23, 0xe3, 0x22, 0x26, 0x66, 0xf7, 0x00, 0xa7, 0x55, 0x4c, 0x72, 0xee, 0x10, 0xdd, 0x01, 0x50,
		0x2d, 0x7a, 0xe1, 0xa9, 0x39, 0x39, 0xde, 0x20, 0x0d, 0x21, 0x20, 0xbd, 0x49, 0x6c, 0x60, 0xb3,
		0x8c, 0x01, 0x01, 0x00, 0x00, 0xff, 0xff, 0xae, 0x65, 0xce, 0x7d, 0xff, 0x00, 0x00, 0x00,
	},
	// google/protobuf/wrappers.proto
	[]byte{
		0x1f, 0x8b, 0x08, 0x00, 0x00, 0x00, 0x00, 0x00, 0x02, 0xff, 0xe2, 0x92, 0x4b, 0xcf, 0xcf, 0x4f,
		0xcf, 0x49, 0xd5, 0x2f, 0x28, 0xca, 0x2f, 0xc9, 0x4f, 0x2a, 0x4d, 0xd3, 0x2f, 0x2f, 0x4a, 0x2c,
		0x28, 0x48, 0x2d, 0x2a, 0xd6, 0x03, 0x8b, 0x08, 0xf1, 0x43, 0xe4, 0xf5, 0x60, 0xf2, 0x4a, 0xca,
		0x5c, 0xdc, 0x2e, 0xf9, 0xa5, 0x49, 0x39, 0xa9, 0x61, 0x89, 0x39, 0xa5, 0xa9, 0x42, 0x22, 0x5c,
		0xac, 0x65, 0x20, 0x86, 0x04, 0xa3, 0x02, 0xa3, 0x06, 0x63, 0x10, 0x84, 0xa3, 0xa4, 0xc4, 0xc5,
		0xe5, 0x96, 0x93, 0x9f, 0x58, 0x82, 0x45, 0x0d, 0x13, 0x92, 0x1a, 0xcf, 0xbc, 0x12, 0x33, 0x13,
		0x2c, 0x6a, 0x98, 0x61, 0x6a, 0x94, 0xb9, 0xb8, 0x43, 0x71, 0x29, 0x62, 0x41, 0x35, 0xc8, 0xd8,
		0x08, 0x8b, 0x1a, 0x56, 0x34, 0x83, 0xb0, 0x2a, 0xe2, 0x85, 0x29, 0x52, 0xe4, 0xe2, 0x74, 0xca,
		0xcf, 0xcf, 0xc1, 0xa2, 0x84, 0x03, 0xc9, 0x9c, 0xe0, 0x92, 0xa2, 0xcc, 0xbc, 0x74, 0x2c, 0x8a,
		0x38, 0x91, 0x1c, 0xe4, 0x54, 0x59, 0x92, 0x5a, 0x8c, 0x45, 0x0d, 0x0f, 0x54, 0x8d, 0x53, 0x33,
		0x23, 0x97, 0x70, 0x72, 0x7e, 0xae, 0x1e, 0x5a, 0xf0, 0x3a, 0xf1, 0x86, 0x43, 0xc3, 0x3f, 0x00,
		0x24, 0x12, 0xc0, 0x18, 0x65, 0x08, 0x55, 0x91, 0x9e, 0x9f, 0x93, 0x98, 0x97, 0xae, 0x97, 0x5f,
		0x94, 0x8e, 0x88, 0xab, 0x92, 0xca, 0x82, 0xd4, 0x62, 0xfd, 0xec, 0xbc, 0xfc, 0xf2, 0x3c, 0x78,
		0xbc, 0x15, 0x24, 0xfd, 0x60, 0x64, 0x5c, 0xc4, 0xc4, 0xec, 0x1e, 0xe0, 0xb4, 0x8a, 0x49, 0xce,
		0x1d, 0xa2, 0x39, 0x00, 0xaa, 0x43, 0x2f, 0x3c, 0x35, 0x27, 0xc7, 0x1b, 0xa4, 0x3e, 0x04, 0xa4,
		0x35, 0x89, 0x0d, 0x6c, 0x94, 0x31, 0x20, 0x00, 0x00, 0xff, 0xff, 0x3c, 0x92, 0x48, 0x30, 0x06,
		0x02, 0x00, 0x00,
	},
}
