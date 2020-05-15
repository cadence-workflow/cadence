(window.webpackJsonp=window.webpackJsonp||[]).push([[61],{399:function(t,e,r){"use strict";r.r(e);var a=r(8),n=Object(a.a)({},(function(){var t=this,e=t.$createElement,r=t._self._c||e;return r("ContentSlotsDistributor",{attrs:{"slot-key":t.$parent.slotKey}},[r("h1",{attrs:{id:"tracing-and-context-propagation"}},[r("a",{staticClass:"header-anchor",attrs:{href:"#tracing-and-context-propagation"}},[t._v("#")]),t._v(" Tracing and context propagation")]),t._v(" "),r("h2",{attrs:{id:"tracing"}},[r("a",{staticClass:"header-anchor",attrs:{href:"#tracing"}},[t._v("#")]),t._v(" Tracing")]),t._v(" "),r("p",[t._v("The Go client provides distributed tracing support through "),r("a",{attrs:{href:"https://opentracing.io/",target:"_blank",rel:"noopener noreferrer"}},[t._v("OpenTracing"),r("OutboundLink")],1),t._v(". Tracing can be\nconfigured by providing an "),r("a",{attrs:{href:"https://godoc.org/github.com/opentracing/opentracing-go#Tracer",target:"_blank",rel:"noopener noreferrer"}},[t._v("opentracing.Tracer"),r("OutboundLink")],1),t._v("\nimplementation in "),r("a",{attrs:{href:"https://godoc.org/go.uber.org/cadence/internal#ClientOptions",target:"_blank",rel:"noopener noreferrer"}},[t._v("ClientOptions"),r("OutboundLink")],1),t._v("\nand "),r("a",{attrs:{href:"https://godoc.org/go.uber.org/cadence/internal#WorkerOptions",target:"_blank",rel:"noopener noreferrer"}},[t._v("WorkerOptions"),r("OutboundLink")],1),t._v(" during client and "),r("Term",{attrs:{term:"worker"}}),t._v(" instantiation,\nrespectively. Tracing allows\nyou to view the call graph of a "),r("Term",{attrs:{term:"workflow"}}),t._v(" along with its "),r("Term",{attrs:{term:"activity",show:"activities"}}),t._v(", child "),r("Term",{attrs:{term:"workflow",show:"workflows"}}),t._v(" etc. For more details on how to\nconfigure and leverage tracing, see the "),r("a",{attrs:{href:"https://opentracing.io/docs/getting-started/",target:"_blank",rel:"noopener noreferrer"}},[t._v("OpenTracing documentation"),r("OutboundLink")],1),t._v(".\nThe OpenTracing support has been validated using "),r("a",{attrs:{href:"https://www.jaegertracing.io/",target:"_blank",rel:"noopener noreferrer"}},[t._v("Jaeger"),r("OutboundLink")],1),t._v(", but other implementations\nmentioned "),r("a",{attrs:{href:"https://opentracing.io/docs/supported-tracers/",target:"_blank",rel:"noopener noreferrer"}},[t._v("here"),r("OutboundLink")],1),t._v(" should also work. Tracing support utilizes generic context\npropagation support provided by the client.")],1),t._v(" "),r("h2",{attrs:{id:"context-propagation"}},[r("a",{staticClass:"header-anchor",attrs:{href:"#context-propagation"}},[t._v("#")]),t._v(" Context Propagation")]),t._v(" "),r("p",[t._v("We provide a standard way to propagate custom context across a "),r("Term",{attrs:{term:"workflow"}}),t._v(".\n"),r("a",{attrs:{href:"https://godoc.org/go.uber.org/cadence/internal#ClientOptions",target:"_blank",rel:"noopener noreferrer"}},[t._v("ClientOptions"),r("OutboundLink")],1),t._v(" and "),r("a",{attrs:{href:"https://godoc.org/go.uber.org/cadence/internal#WorkerOptions",target:"_blank",rel:"noopener noreferrer"}},[t._v("WorkerOptions"),r("OutboundLink")],1),t._v("\nallow configuring a context propagator. The context propagator extracts and passes on information present in the "),r("code",[t._v("context.Context")]),t._v("\nand "),r("code",[t._v("workflow.Context")]),t._v(" objects across the "),r("Term",{attrs:{term:"workflow"}}),t._v(". Once a context propagator is configured, you should be able to access the required values\nin the context objects as you would normally do in Go.\nFor a sample, the Go client implements a "),r("a",{attrs:{href:"https://github.com/uber-go/cadence-client/blob/master/internal/tracer.go",target:"_blank",rel:"noopener noreferrer"}},[t._v("tracing context propagator"),r("OutboundLink")],1),t._v(".")],1),t._v(" "),r("h3",{attrs:{id:"server-side-headers-support"}},[r("a",{staticClass:"header-anchor",attrs:{href:"#server-side-headers-support"}},[t._v("#")]),t._v(" Server-Side Headers Support")]),t._v(" "),r("p",[t._v("On the server side, Cadence provides a mechanism to propagate what it calls headers across different "),r("Term",{attrs:{term:"workflow"}}),t._v("\ntransitions.")],1),t._v(" "),r("div",{staticClass:"language-go extra-class"},[r("pre",{pre:!0,attrs:{class:"language-go"}},[r("code",[r("span",{pre:!0,attrs:{class:"token keyword"}},[t._v("struct")]),t._v(" Header "),r("span",{pre:!0,attrs:{class:"token punctuation"}},[t._v("{")]),t._v("\n  "),r("span",{pre:!0,attrs:{class:"token number"}},[t._v("10")]),r("span",{pre:!0,attrs:{class:"token punctuation"}},[t._v(":")]),t._v(" optional "),r("span",{pre:!0,attrs:{class:"token keyword"}},[t._v("map")]),r("span",{pre:!0,attrs:{class:"token operator"}},[t._v("<")]),r("span",{pre:!0,attrs:{class:"token builtin"}},[t._v("string")]),r("span",{pre:!0,attrs:{class:"token punctuation"}},[t._v(",")]),t._v(" binary"),r("span",{pre:!0,attrs:{class:"token operator"}},[t._v(">")]),t._v(" fields\n"),r("span",{pre:!0,attrs:{class:"token punctuation"}},[t._v("}")]),t._v("\n")])])]),r("p",[t._v("The client leverages this to pass around selected context information. "),r("a",{attrs:{href:"https://godoc.org/go.uber.org/cadence/internal#HeaderReader",target:"_blank",rel:"noopener noreferrer"}},[t._v("HeaderReader"),r("OutboundLink")],1),t._v("\nand "),r("a",{attrs:{href:"https://godoc.org/go.uber.org/cadence/internal#HeaderWriter",target:"_blank",rel:"noopener noreferrer"}},[t._v("HeaderWriter"),r("OutboundLink")],1),t._v(" are interfaces\nthat allow reading and writing to the Cadence server headers. The client already provides "),r("a",{attrs:{href:"https://github.com/uber-go/cadence-client/blob/master/internal/headers.go",target:"_blank",rel:"noopener noreferrer"}},[t._v("implementations"),r("OutboundLink")],1),t._v("\nfor these. "),r("code",[t._v("HeaderWriter")]),t._v(" sets a field in the header. Headers is a map, so setting a value for the the same key\nmultiple times will overwrite the previous values. "),r("code",[t._v("HeaderReader")]),t._v(" iterates through the headers map and runs the\nprovided handler function on each key/value pair, allowing you to deal with the fields you are interested in.")]),t._v(" "),r("div",{staticClass:"language-go extra-class"},[r("pre",{pre:!0,attrs:{class:"language-go"}},[r("code",[r("span",{pre:!0,attrs:{class:"token keyword"}},[t._v("type")]),t._v(" HeaderWriter "),r("span",{pre:!0,attrs:{class:"token keyword"}},[t._v("interface")]),t._v(" "),r("span",{pre:!0,attrs:{class:"token punctuation"}},[t._v("{")]),t._v("\n  "),r("span",{pre:!0,attrs:{class:"token function"}},[t._v("Set")]),r("span",{pre:!0,attrs:{class:"token punctuation"}},[t._v("(")]),r("span",{pre:!0,attrs:{class:"token builtin"}},[t._v("string")]),r("span",{pre:!0,attrs:{class:"token punctuation"}},[t._v(",")]),t._v(" "),r("span",{pre:!0,attrs:{class:"token punctuation"}},[t._v("[")]),r("span",{pre:!0,attrs:{class:"token punctuation"}},[t._v("]")]),r("span",{pre:!0,attrs:{class:"token builtin"}},[t._v("byte")]),r("span",{pre:!0,attrs:{class:"token punctuation"}},[t._v(")")]),t._v("\n"),r("span",{pre:!0,attrs:{class:"token punctuation"}},[t._v("}")]),t._v("\n\n"),r("span",{pre:!0,attrs:{class:"token keyword"}},[t._v("type")]),t._v(" HeaderReader "),r("span",{pre:!0,attrs:{class:"token keyword"}},[t._v("interface")]),t._v(" "),r("span",{pre:!0,attrs:{class:"token punctuation"}},[t._v("{")]),t._v("\n  "),r("span",{pre:!0,attrs:{class:"token function"}},[t._v("ForEachKey")]),r("span",{pre:!0,attrs:{class:"token punctuation"}},[t._v("(")]),t._v("handler "),r("span",{pre:!0,attrs:{class:"token keyword"}},[t._v("func")]),r("span",{pre:!0,attrs:{class:"token punctuation"}},[t._v("(")]),r("span",{pre:!0,attrs:{class:"token builtin"}},[t._v("string")]),r("span",{pre:!0,attrs:{class:"token punctuation"}},[t._v(",")]),t._v(" "),r("span",{pre:!0,attrs:{class:"token punctuation"}},[t._v("[")]),r("span",{pre:!0,attrs:{class:"token punctuation"}},[t._v("]")]),r("span",{pre:!0,attrs:{class:"token builtin"}},[t._v("byte")]),r("span",{pre:!0,attrs:{class:"token punctuation"}},[t._v(")")]),t._v(" "),r("span",{pre:!0,attrs:{class:"token builtin"}},[t._v("error")]),r("span",{pre:!0,attrs:{class:"token punctuation"}},[t._v(")")]),t._v(" "),r("span",{pre:!0,attrs:{class:"token builtin"}},[t._v("error")]),t._v("\n"),r("span",{pre:!0,attrs:{class:"token punctuation"}},[t._v("}")]),t._v("\n")])])]),r("h3",{attrs:{id:"context-propagators"}},[r("a",{staticClass:"header-anchor",attrs:{href:"#context-propagators"}},[t._v("#")]),t._v(" Context Propagators")]),t._v(" "),r("p",[t._v("Context propagators require implementing the following four methods to propagate selected context across a workflow:")]),t._v(" "),r("ul",[r("li",[r("code",[t._v("Inject")]),t._v(" is meant to pick out the context keys of interest from a Go "),r("a",{attrs:{href:"https://golang.org/pkg/context/#Context",target:"_blank",rel:"noopener noreferrer"}},[t._v("context.Context"),r("OutboundLink")],1),t._v(" object and write that into the\nheaders using the "),r("a",{attrs:{href:"https://godoc.org/go.uber.org/cadence/internal#HeaderWriter",target:"_blank",rel:"noopener noreferrer"}},[t._v("HeaderWriter"),r("OutboundLink")],1),t._v(" interface")]),t._v(" "),r("li",[r("code",[t._v("InjectFromWorkflow")]),t._v(" is the same as above, but operates on a "),r("a",{attrs:{href:"https://godoc.org/go.uber.org/cadence/internal#Context",target:"_blank",rel:"noopener noreferrer"}},[t._v("workflow.Context"),r("OutboundLink")],1),t._v(" object")]),t._v(" "),r("li",[r("code",[t._v("Extract")]),t._v(" reads the headers and places the information of interest back into the "),r("a",{attrs:{href:"https://golang.org/pkg/context/#Context",target:"_blank",rel:"noopener noreferrer"}},[t._v("context.Context"),r("OutboundLink")],1),t._v(" object")]),t._v(" "),r("li",[r("code",[t._v("ExtractToWorkflow")]),t._v(" is the same as above, but operates on a "),r("a",{attrs:{href:"https://godoc.org/go.uber.org/cadence/internal#Context",target:"_blank",rel:"noopener noreferrer"}},[t._v("workflow.Context"),r("OutboundLink")],1),t._v(" object")])]),t._v(" "),r("p",[t._v("The "),r("a",{attrs:{href:"https://github.com/uber-go/cadence-client/blob/master/internal/tracer.go",target:"_blank",rel:"noopener noreferrer"}},[t._v("tracing context propagator"),r("OutboundLink")],1),t._v("\nshows a sample implementation of context propagation.")]),t._v(" "),r("div",{staticClass:"language-go extra-class"},[r("pre",{pre:!0,attrs:{class:"language-go"}},[r("code",[r("span",{pre:!0,attrs:{class:"token keyword"}},[t._v("type")]),t._v(" ContextPropagator "),r("span",{pre:!0,attrs:{class:"token keyword"}},[t._v("interface")]),t._v(" "),r("span",{pre:!0,attrs:{class:"token punctuation"}},[t._v("{")]),t._v("\n  "),r("span",{pre:!0,attrs:{class:"token function"}},[t._v("Inject")]),r("span",{pre:!0,attrs:{class:"token punctuation"}},[t._v("(")]),t._v("context"),r("span",{pre:!0,attrs:{class:"token punctuation"}},[t._v(".")]),t._v("Context"),r("span",{pre:!0,attrs:{class:"token punctuation"}},[t._v(",")]),t._v(" HeaderWriter"),r("span",{pre:!0,attrs:{class:"token punctuation"}},[t._v(")")]),t._v(" "),r("span",{pre:!0,attrs:{class:"token builtin"}},[t._v("error")]),t._v("\n\n  "),r("span",{pre:!0,attrs:{class:"token function"}},[t._v("Extract")]),r("span",{pre:!0,attrs:{class:"token punctuation"}},[t._v("(")]),t._v("context"),r("span",{pre:!0,attrs:{class:"token punctuation"}},[t._v(".")]),t._v("Context"),r("span",{pre:!0,attrs:{class:"token punctuation"}},[t._v(",")]),t._v(" HeaderReader"),r("span",{pre:!0,attrs:{class:"token punctuation"}},[t._v(")")]),t._v(" "),r("span",{pre:!0,attrs:{class:"token punctuation"}},[t._v("(")]),t._v("context"),r("span",{pre:!0,attrs:{class:"token punctuation"}},[t._v(".")]),t._v("Context"),r("span",{pre:!0,attrs:{class:"token punctuation"}},[t._v(",")]),t._v(" "),r("span",{pre:!0,attrs:{class:"token builtin"}},[t._v("error")]),r("span",{pre:!0,attrs:{class:"token punctuation"}},[t._v(")")]),t._v("\n\n  "),r("span",{pre:!0,attrs:{class:"token function"}},[t._v("InjectFromWorkflow")]),r("span",{pre:!0,attrs:{class:"token punctuation"}},[t._v("(")]),t._v("Context"),r("span",{pre:!0,attrs:{class:"token punctuation"}},[t._v(",")]),t._v(" HeaderWriter"),r("span",{pre:!0,attrs:{class:"token punctuation"}},[t._v(")")]),t._v(" "),r("span",{pre:!0,attrs:{class:"token builtin"}},[t._v("error")]),t._v("\n\n  "),r("span",{pre:!0,attrs:{class:"token function"}},[t._v("ExtractToWorkflow")]),r("span",{pre:!0,attrs:{class:"token punctuation"}},[t._v("(")]),t._v("Context"),r("span",{pre:!0,attrs:{class:"token punctuation"}},[t._v(",")]),t._v(" HeaderReader"),r("span",{pre:!0,attrs:{class:"token punctuation"}},[t._v(")")]),t._v(" "),r("span",{pre:!0,attrs:{class:"token punctuation"}},[t._v("(")]),t._v("Context"),r("span",{pre:!0,attrs:{class:"token punctuation"}},[t._v(",")]),t._v(" "),r("span",{pre:!0,attrs:{class:"token builtin"}},[t._v("error")]),r("span",{pre:!0,attrs:{class:"token punctuation"}},[t._v(")")]),t._v("\n"),r("span",{pre:!0,attrs:{class:"token punctuation"}},[t._v("}")]),t._v("\n")])])]),r("h2",{attrs:{id:"q-a"}},[r("a",{staticClass:"header-anchor",attrs:{href:"#q-a"}},[t._v("#")]),t._v(" Q & A")]),t._v(" "),r("h3",{attrs:{id:"is-there-a-complete-example"}},[r("a",{staticClass:"header-anchor",attrs:{href:"#is-there-a-complete-example"}},[t._v("#")]),t._v(" Is there a complete example?")]),t._v(" "),r("p",[t._v("The "),r("a",{attrs:{href:"https://github.com/uber-common/cadence-samples/blob/master/cmd/samples/recipes/ctxpropagation/workflow.go",target:"_blank",rel:"noopener noreferrer"}},[t._v("context propagation sample"),r("OutboundLink")],1),t._v("\nconfigures a custom context propagator and shows context propagation of custom keys across a "),r("Term",{attrs:{term:"workflow"}}),t._v(" and an "),r("Term",{attrs:{term:"activity"}}),t._v(".")],1),t._v(" "),r("h3",{attrs:{id:"can-i-configure-multiple-context-propagators"}},[r("a",{staticClass:"header-anchor",attrs:{href:"#can-i-configure-multiple-context-propagators"}},[t._v("#")]),t._v(" Can I configure multiple context propagators?")]),t._v(" "),r("p",[t._v("Yes, we recommended that you configure multiple context propagators with each propagator meant to propagate a particular type of context.")])])}),[],!1,null,null,null);e.default=n.exports}}]);