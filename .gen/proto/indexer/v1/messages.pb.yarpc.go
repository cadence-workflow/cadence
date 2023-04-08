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
// source: uber/cadence/indexer/v1/messages.proto

package indexerv1

var yarpcFileDescriptorClosure60256a432328b016 = [][]byte{
	// uber/cadence/indexer/v1/messages.proto
	[]byte{
		0x1f, 0x8b, 0x08, 0x00, 0x00, 0x00, 0x00, 0x00, 0x02, 0xff, 0x8c, 0x53, 0xdb, 0x6e, 0xd3, 0x30,
		0x18, 0x6e, 0x9a, 0xf5, 0xf4, 0x67, 0x42, 0xc5, 0x20, 0x88, 0x36, 0x81, 0xb2, 0x49, 0xa0, 0x0a,
		0x84, 0xa3, 0x76, 0x17, 0x1c, 0x76, 0xb5, 0xa9, 0x81, 0x56, 0x6c, 0x13, 0xf2, 0x2a, 0x18, 0x08,
		0x29, 0xb8, 0x8d, 0x57, 0xac, 0x35, 0x76, 0xe5, 0xb8, 0xe9, 0xfa, 0x18, 0x3c, 0x00, 0xef, 0x8a,
		0x1c, 0x67, 0xa2, 0x9d, 0x10, 0x70, 0x67, 0x7f, 0x47, 0xf9, 0xcf, 0x1f, 0x78, 0xba, 0x18, 0x33,
		0x15, 0x4e, 0x68, 0xc2, 0xc4, 0x84, 0x85, 0x5c, 0x24, 0xec, 0x9a, 0xa9, 0x30, 0xef, 0x86, 0x29,
		0xcb, 0x32, 0x3a, 0x65, 0x19, 0x9e, 0x2b, 0xa9, 0x25, 0x7a, 0x9c, 0x31, 0x95, 0x33, 0x85, 0x8d,
		0x1c, 0x97, 0x72, 0x5c, 0xca, 0x71, 0xde, 0xdd, 0x09, 0x36, 0x72, 0xe8, 0x9c, 0x9b, 0x8c, 0x89,
		0x4c, 0x53, 0x29, 0x6c, 0xc2, 0xfe, 0x4f, 0x17, 0x1a, 0xa7, 0x36, 0x14, 0x9d, 0xc1, 0x76, 0x99,
		0x1f, 0xeb, 0xd5, 0x9c, 0xf9, 0x4e, 0xe0, 0x74, 0xee, 0xf4, 0x9e, 0xe3, 0xbf, 0x97, 0xe0, 0xd2,
		0x3e, 0x5a, 0xcd, 0x19, 0xf1, 0xd2, 0xdf, 0x17, 0xb4, 0x0b, 0xad, 0x44, 0xa6, 0x94, 0x8b, 0x98,
		0x27, 0x7e, 0x35, 0x70, 0x3a, 0x2d, 0xd2, 0xb4, 0xc0, 0x30, 0x41, 0x5f, 0x01, 0x2d, 0xa5, 0xba,
		0xba, 0x9c, 0xc9, 0x65, 0xcc, 0xae, 0xd9, 0x64, 0xa1, 0xb9, 0x14, 0xbe, 0x1b, 0x38, 0x1d, 0xaf,
		0xf7, 0xe2, 0x8f, 0x95, 0x74, 0xce, 0x4d, 0xdd, 0xa7, 0xd2, 0x15, 0xdd, 0x98, 0xc8, 0xdd, 0xe5,
		0x6d, 0x08, 0xf9, 0xd0, 0xc8, 0x99, 0xca, 0x4c, 0xe4, 0x56, 0xe0, 0x74, 0x5c, 0x72, 0x73, 0x45,
		0xef, 0xa1, 0x7e, 0xc9, 0xd9, 0x2c, 0xc9, 0xfc, 0x5a, 0xe0, 0x76, 0xbc, 0xde, 0xc1, 0x7f, 0x3e,
		0x0f, 0xbf, 0x2d, 0x5c, 0x91, 0xd0, 0x6a, 0x45, 0xca, 0x88, 0x9d, 0x6f, 0xe0, 0xad, 0xc1, 0xa8,
		0x0d, 0xee, 0x15, 0x5b, 0x15, 0x73, 0x6b, 0x11, 0x73, 0x44, 0x87, 0x50, 0xcb, 0xe9, 0x6c, 0xc1,
		0x8a, 0xe7, 0x7b, 0xbd, 0x27, 0xff, 0x2a, 0x2b, 0xd2, 0x88, 0xf5, 0xbc, 0xa9, 0xbe, 0x72, 0xf6,
		0x7f, 0x38, 0x50, 0x2b, 0x40, 0xb4, 0x07, 0x5e, 0xa6, 0x15, 0x17, 0xd3, 0x38, 0xa1, 0x9a, 0xda,
		0x92, 0x41, 0x85, 0x80, 0x05, 0xfb, 0x54, 0x53, 0xb4, 0x0b, 0x4d, 0x2e, 0xb4, 0xe5, 0x4d, 0xa1,
		0x3b, 0xa8, 0x90, 0x06, 0x17, 0xba, 0x20, 0x1f, 0x41, 0x6b, 0x2c, 0xe5, 0xcc, 0xb2, 0x66, 0xce,
		0xcd, 0x41, 0x85, 0x34, 0x0d, 0x54, 0xd0, 0x7b, 0xe0, 0x8d, 0xb9, 0xa0, 0x6a, 0x65, 0x05, 0x66,
		0x6a, 0xdb, 0x26, 0xde, 0x82, 0x46, 0x72, 0x5c, 0x87, 0x2d, 0xc3, 0x3d, 0xbb, 0x00, 0x6f, 0xed,
		0x9b, 0x23, 0x1f, 0xee, 0x9f, 0x46, 0xe7, 0xe7, 0x47, 0xef, 0xa2, 0x78, 0xf4, 0xf9, 0x43, 0x14,
		0x0f, 0xcf, 0x3e, 0x1e, 0x9d, 0x0c, 0xfb, 0xed, 0x0a, 0x7a, 0x00, 0xe8, 0x16, 0xd3, 0x8f, 0x2e,
		0xda, 0x0e, 0x7a, 0x08, 0xf7, 0x36, 0xf0, 0x7e, 0x74, 0x12, 0x8d, 0xa2, 0x76, 0xf5, 0xf8, 0xf5,
		0x97, 0x97, 0x53, 0xae, 0xbf, 0x2f, 0xc6, 0x78, 0x22, 0xd3, 0x70, 0x63, 0x79, 0xf1, 0x94, 0x89,
		0xb0, 0xd8, 0xd9, 0xb5, 0xff, 0xe1, 0xb0, 0x3c, 0xe6, 0xdd, 0x71, 0xbd, 0xe0, 0x0e, 0x7e, 0x05,
		0x00, 0x00, 0xff, 0xff, 0x7f, 0xac, 0xe2, 0x7e, 0x3b, 0x03, 0x00, 0x00,
	},
	// uber/cadence/api/v1/common.proto
	[]byte{
		0x1f, 0x8b, 0x08, 0x00, 0x00, 0x00, 0x00, 0x00, 0x02, 0xff, 0xc4, 0x55, 0x4f, 0x73, 0xdb, 0x44,
		0x14, 0x47, 0x71, 0xec, 0xb4, 0xcf, 0x6e, 0x6a, 0xb6, 0x34, 0x75, 0xcc, 0x00, 0x1e, 0x73, 0xc0,
		0x30, 0x8c, 0x34, 0x49, 0x2f, 0x40, 0x87, 0x61, 0x92, 0xd8, 0xa1, 0x6a, 0x69, 0xe2, 0x91, 0x3d,
		0xed, 0x94, 0x03, 0x9a, 0xb5, 0xf4, 0xe4, 0x2e, 0x96, 0x76, 0x35, 0xab, 0x95, 0x12, 0xdf, 0xf8,
		0x40, 0x1c, 0xf8, 0x4a, 0x7c, 0x13, 0x66, 0xa5, 0x75, 0x6c, 0x17, 0x9a, 0x5e, 0x98, 0xe9, 0x4d,
		0xfb, 0x7e, 0x7f, 0xde, 0xef, 0x69, 0xf6, 0x0f, 0xf4, 0xf2, 0x19, 0x4a, 0x27, 0xa0, 0x21, 0xf2,
		0x00, 0x1d, 0x9a, 0x32, 0xa7, 0x38, 0x72, 0x02, 0x91, 0x24, 0x82, 0xdb, 0xa9, 0x14, 0x4a, 0x90,
		0x6e, 0x86, 0xb2, 0x40, 0x69, 0x6b, 0xa2, 0x6d, 0x88, 0x36, 0x4d, 0x99, 0x5d, 0x1c, 0x75, 0x3f,
		0x9f, 0x0b, 0x31, 0x8f, 0xd1, 0x29, 0x99, 0xb3, 0x3c, 0x72, 0xc2, 0x5c, 0x52, 0xc5, 0x56, 0xda,
		0xfe, 0x73, 0xf8, 0xf8, 0x95, 0x90, 0x8b, 0x28, 0x16, 0x57, 0xa3, 0x6b, 0x0c, 0x72, 0x0d, 0x91,
		0x2f, 0xa0, 0x79, 0x65, 0x8a, 0x3e, 0x0b, 0x3b, 0x56, 0xcf, 0x1a, 0xdc, 0xf5, 0x60, 0x55, 0x72,
		0x43, 0xf2, 0x10, 0x1a, 0x32, 0xe7, 0x1a, 0xdb, 0x29, 0xb1, 0xba, 0xcc, 0xb9, 0x1b, 0xf6, 0xfb,
		0xd0, 0x5a, 0x99, 0x4d, 0x97, 0x29, 0x12, 0x02, 0xbb, 0x9c, 0x26, 0x68, 0x0c, 0xca, 0x6f, 0xcd,
		0x39, 0x09, 0x14, 0x2b, 0x98, 0x5a, 0xbe, 0x93, 0xf3, 0x19, 0xec, 0x8d, 0xe9, 0x32, 0x16, 0x34,
		0xd4, 0x70, 0x48, 0x15, 0x2d, 0xe1, 0x96, 0x57, 0x7e, 0xf7, 0x9f, 0xc0, 0xde, 0x39, 0x65, 0x71,
		0x2e, 0x91, 0x1c, 0x40, 0x43, 0x22, 0xcd, 0x04, 0x37, 0x7a, 0xb3, 0x22, 0x1d, 0xd8, 0x0b, 0x51,
		0x51, 0x16, 0x67, 0x65, 0xc2, 0x96, 0xb7, 0x5a, 0xf6, 0xff, 0xb4, 0x60, 0xf7, 0x05, 0x26, 0x82,
		0x0c, 0xa1, 0x11, 0x31, 0x8c, 0xc3, 0xac, 0x63, 0xf5, 0x6a, 0x83, 0xe6, 0xf1, 0xb7, 0xf6, 0xbb,
		0x7f, 0xa3, 0xad, 0x15, 0xf6, 0x79, 0x49, 0x1f, 0x71, 0x25, 0x97, 0x9e, 0xd1, 0x76, 0x7f, 0x83,
		0xe6, 0x46, 0x99, 0xb4, 0xa1, 0xb6, 0xc0, 0xa5, 0x09, 0xa3, 0x3f, 0xc9, 0xf7, 0x50, 0x2f, 0x68,
		0x9c, 0x63, 0x99, 0xa3, 0x79, 0xfc, 0xe5, 0x6d, 0x5d, 0xcc, 0xd0, 0x5e, 0xa5, 0xf8, 0x61, 0xe7,
		0x3b, 0xab, 0xff, 0x97, 0x05, 0x8d, 0xa7, 0x48, 0x43, 0x94, 0xe4, 0xfc, 0xad, 0xc0, 0xf6, 0x6d,
		0x56, 0x95, 0xe6, 0x83, 0x44, 0xfe, 0xdb, 0x82, 0xf6, 0x04, 0xa9, 0x0c, 0xde, 0x9c, 0x28, 0x25,
		0xd9, 0x2c, 0x57, 0x98, 0x91, 0x08, 0xf6, 0x19, 0x0f, 0xf1, 0x1a, 0x43, 0x7f, 0x6b, 0x88, 0x9f,
		0x6e, 0x33, 0x7f, 0xdb, 0xc5, 0x76, 0x2b, 0x8b, 0xcd, 0xa9, 0xee, 0xb1, 0xcd, 0x5a, 0x17, 0x81,
		0xfc, 0x9b, 0xf4, 0xff, 0xcf, 0x98, 0xc0, 0x9d, 0x21, 0x55, 0xf4, 0x34, 0x16, 0x33, 0xf2, 0x02,
		0xee, 0x21, 0x0f, 0x44, 0xc8, 0xf8, 0xdc, 0x57, 0xcb, 0xb4, 0xda, 0xca, 0xfb, 0xc7, 0x83, 0xdb,
		0x2c, 0x47, 0x46, 0xa0, 0x8f, 0x80, 0xd7, 0xc2, 0x8d, 0xd5, 0xcd, 0x8e, 0xdf, 0xd9, 0xd8, 0xf1,
		0xe3, 0xea, 0x94, 0xa2, 0x7c, 0x89, 0x32, 0x63, 0x82, 0xbb, 0x3c, 0x12, 0x9a, 0xc8, 0x92, 0x34,
		0x5e, 0x9d, 0x1c, 0xfd, 0x4d, 0xbe, 0x82, 0xfb, 0x11, 0x52, 0x95, 0x4b, 0xf4, 0x8b, 0x8a, 0x6a,
		0x4e, 0xe8, 0xbe, 0x29, 0x1b, 0x83, 0xfe, 0x73, 0x78, 0x34, 0xc9, 0xd3, 0x54, 0x48, 0x85, 0xe1,
		0x59, 0xcc, 0x90, 0x2b, 0x83, 0x64, 0xfa, 0x70, 0xcf, 0x85, 0x9f, 0x85, 0x0b, 0xe3, 0x5c, 0x9f,
		0x8b, 0x49, 0xb8, 0x20, 0x87, 0x70, 0xe7, 0x77, 0x5a, 0xd0, 0x12, 0xa8, 0x3c, 0xf7, 0xf4, 0x7a,
		0x12, 0x2e, 0xfa, 0x7f, 0xd4, 0xa0, 0xe9, 0xa1, 0x92, 0xcb, 0xb1, 0x88, 0x59, 0xb0, 0x24, 0x43,
		0x68, 0x33, 0xce, 0x14, 0xa3, 0xb1, 0xcf, 0xb8, 0x42, 0x59, 0xd0, 0x2a, 0x65, 0xf3, 0xf8, 0xd0,
		0xae, 0xee, 0x23, 0x7b, 0x75, 0x1f, 0xd9, 0x43, 0x73, 0x1f, 0x79, 0xf7, 0x8d, 0xc4, 0x35, 0x0a,
		0xe2, 0xc0, 0x83, 0x19, 0x0d, 0x16, 0x22, 0x8a, 0xfc, 0x40, 0x60, 0x14, 0xb1, 0x40, 0xc7, 0x2c,
		0x7b, 0x5b, 0x1e, 0x31, 0xd0, 0xd9, 0x1a, 0xd1, 0x6d, 0x13, 0x7a, 0xcd, 0x92, 0x3c, 0x59, 0xb7,
		0xad, 0xbd, 0xb7, 0xad, 0x91, 0xdc, 0xb4, 0xfd, 0x7a, 0xed, 0x42, 0x95, 0xc2, 0x24, 0x55, 0x59,
		0x67, 0xb7, 0x67, 0x0d, 0xea, 0x37, 0xd4, 0x13, 0x53, 0x26, 0x3f, 0xc2, 0xa7, 0x5c, 0x70, 0x5f,
		0xea, 0xd1, 0xe9, 0x2c, 0x46, 0x1f, 0xa5, 0x14, 0xd2, 0xaf, 0xee, 0xa0, 0xac, 0x53, 0xef, 0xd5,
		0x06, 0x77, 0xbd, 0x0e, 0x17, 0xdc, 0x5b, 0x31, 0x46, 0x9a, 0xe0, 0x55, 0x38, 0x79, 0x06, 0x0f,
		0xf0, 0x3a, 0x65, 0x55, 0x90, 0x75, 0xe4, 0xc6, 0xfb, 0x22, 0x93, 0xb5, 0x6a, 0x95, 0xfa, 0x9b,
		0x2b, 0x68, 0x6d, 0xee, 0x29, 0x72, 0x08, 0x0f, 0x47, 0x17, 0x67, 0x97, 0x43, 0xf7, 0xe2, 0x67,
		0x7f, 0xfa, 0x7a, 0x3c, 0xf2, 0xdd, 0x8b, 0x97, 0x27, 0xbf, 0xb8, 0xc3, 0xf6, 0x47, 0xa4, 0x0b,
		0x07, 0xdb, 0xd0, 0xf4, 0xa9, 0xe7, 0x9e, 0x4f, 0xbd, 0x57, 0x6d, 0x8b, 0x1c, 0x00, 0xd9, 0xc6,
		0x9e, 0x4d, 0x2e, 0x2f, 0xda, 0x3b, 0xa4, 0x03, 0x9f, 0x6c, 0xd7, 0xc7, 0xde, 0xe5, 0xf4, 0xf2,
		0x71, 0xbb, 0x76, 0xfa, 0x1a, 0x1e, 0x05, 0x22, 0xf9, 0xaf, 0x4d, 0x7e, 0xda, 0x3c, 0x2b, 0x5f,
		0xa9, 0xb1, 0x1e, 0x60, 0x6c, 0xfd, 0xea, 0xcc, 0x99, 0x7a, 0x93, 0xcf, 0xec, 0x40, 0x24, 0xce,
		0xd6, 0x9b, 0x66, 0xcf, 0x91, 0x57, 0x0f, 0x94, 0x79, 0xde, 0x9e, 0xd0, 0x94, 0x15, 0x47, 0xb3,
		0x46, 0x59, 0x7b, 0xfc, 0x4f, 0x00, 0x00, 0x00, 0xff, 0xff, 0xc3, 0x38, 0x66, 0x5d, 0x02, 0x07,
		0x00, 0x00,
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
}
