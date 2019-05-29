// Copyright (c) 2017 Uber Technologies, Inc.
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in
// all copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN
// THE SOFTWARE.

// Code generated by thriftrw v1.19.1. DO NOT EDIT.
// @generated

package indexer

import (
	shared "github.com/uber/cadence/.gen/go/shared"
	thriftreflect "go.uber.org/thriftrw/thriftreflect"
)

// ThriftModule represents the IDL file used to generate this package.
var ThriftModule = &thriftreflect.ThriftModule{
	Name:     "indexer",
	Package:  "github.com/uber/cadence/.gen/go/indexer",
	FilePath: "indexer.thrift",
	SHA1:     "464fff1b44654c3f41da6b7a95de46593a9776d6",
	Includes: []*thriftreflect.ThriftModule{
		shared.ThriftModule,
	},
	Raw: rawIDL,
}

const rawIDL = "// Copyright (c) 2017 Uber Technologies, Inc.\n//\n// Permission is hereby granted, free of charge, to any person obtaining a copy\n// of this software and associated documentation files (the \"Software\"), to deal\n// in the Software without restriction, including without limitation the rights\n// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell\n// copies of the Software, and to permit persons to whom the Software is\n// furnished to do so, subject to the following conditions:\n//\n// The above copyright notice and this permission notice shall be included in\n// all copies or substantial portions of the Software.\n//\n// THE SOFTWARE IS PROVIDED \"AS IS\", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR\n// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,\n// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE\n// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER\n// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,\n// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN\n// THE SOFTWARE.\n\nnamespace java com.uber.cadence.indexer\n\ninclude \"shared.thrift\"\n\nenum MessageType {\n  Index\n  Delete\n}\n\nenum FieldType {\n  String\n  Int\n  Bool\n  Binary\n}\n\nstruct Field {\n  10: optional FieldType type\n  20: optional string stringData\n  30: optional i64 (js.type = \"Long\") intData\n  40: optional bool boolData\n  50: optional binary binaryData\n}\n\nstruct Message {\n  10: optional MessageType messageType\n  20: optional string domainID\n  30: optional string workflowID\n  40: optional string runID\n  50: optional i64 (js.type = \"Long\") version\n  60: optional map<string,Field> fields\n}"
