(window.webpackJsonp=window.webpackJsonp||[]).push([[52],{377:function(t,a,s){"use strict";s.r(a);var n=s(8),e=Object(n.a)({},(function(){var t=this,a=t.$createElement,s=t._self._c||a;return s("ContentSlotsDistributor",{attrs:{"slot-key":t.$parent.slotKey}},[s("h1",{attrs:{id:"signals"}},[s("a",{staticClass:"header-anchor",attrs:{href:"#signals"}},[t._v("#")]),t._v(" Signals")]),t._v(" "),s("p",[s("Term",{attrs:{term:"signal",show:"Signals"}}),t._v(" provide a mechanism to send data directly to a running "),s("Term",{attrs:{term:"workflow"}}),t._v(". Previously, you had\ntwo options for passing data to the "),s("Term",{attrs:{term:"workflow"}}),t._v(" implementation:")],1),t._v(" "),s("ul",[s("li",[t._v("Via start parameters")]),t._v(" "),s("li",[t._v("As return values from "),s("Term",{attrs:{term:"activity",show:"activities"}})],1)]),t._v(" "),s("p",[t._v("With start parameters, we could only pass in values before "),s("Term",{attrs:{term:"workflow_execution"}}),t._v(" began.")],1),t._v(" "),s("p",[t._v("Return values from "),s("Term",{attrs:{term:"activity",show:"activities"}}),t._v(" allowed us to pass information to a running "),s("Term",{attrs:{term:"workflow"}}),t._v(", but this\napproach comes with its own complications. One major drawback is reliance on polling. This means\nthat the data needs to be stored in a third-party location until it's ready to be picked up by\nthe "),s("Term",{attrs:{term:"activity"}}),t._v(". Further, the lifecycle of this "),s("Term",{attrs:{term:"activity"}}),t._v(" requires management, and the "),s("Term",{attrs:{term:"activity"}}),t._v("\nrequires manual restart if it fails before acquiring the data.")],1),t._v(" "),s("p",[s("Term",{attrs:{term:"signal",show:"Signals"}}),t._v(", on the other hand, provide a fully asynchronous and durable mechanism for providing data to\na running "),s("Term",{attrs:{term:"workflow"}}),t._v(". When a "),s("Term",{attrs:{term:"signal"}}),t._v(" is received for a running "),s("Term",{attrs:{term:"workflow"}}),t._v(", Cadence persists the "),s("Term",{attrs:{term:"event"}}),t._v("\nand the payload in the "),s("Term",{attrs:{term:"workflow"}}),t._v(" history. The "),s("Term",{attrs:{term:"workflow"}}),t._v(" can then process the "),s("Term",{attrs:{term:"signal"}}),t._v(" at any time\nafterwards without the risk of losing the information. The "),s("Term",{attrs:{term:"workflow"}}),t._v(" also has the option to stop\nexecution by blocking on a "),s("Term",{attrs:{term:"signal"}}),t._v(" channel.")],1),t._v(" "),s("div",{staticClass:"language-go extra-class"},[s("pre",{pre:!0,attrs:{class:"language-go"}},[s("code",[s("span",{pre:!0,attrs:{class:"token keyword"}},[t._v("var")]),t._v(" signalVal "),s("span",{pre:!0,attrs:{class:"token builtin"}},[t._v("string")]),t._v("\nsignalChan "),s("span",{pre:!0,attrs:{class:"token operator"}},[t._v(":=")]),t._v(" workflow"),s("span",{pre:!0,attrs:{class:"token punctuation"}},[t._v(".")]),s("span",{pre:!0,attrs:{class:"token function"}},[t._v("GetSignalChannel")]),s("span",{pre:!0,attrs:{class:"token punctuation"}},[t._v("(")]),t._v("ctx"),s("span",{pre:!0,attrs:{class:"token punctuation"}},[t._v(",")]),t._v(" signalName"),s("span",{pre:!0,attrs:{class:"token punctuation"}},[t._v(")")]),t._v("\n\ns "),s("span",{pre:!0,attrs:{class:"token operator"}},[t._v(":=")]),t._v(" workflow"),s("span",{pre:!0,attrs:{class:"token punctuation"}},[t._v(".")]),s("span",{pre:!0,attrs:{class:"token function"}},[t._v("NewSelector")]),s("span",{pre:!0,attrs:{class:"token punctuation"}},[t._v("(")]),t._v("ctx"),s("span",{pre:!0,attrs:{class:"token punctuation"}},[t._v(")")]),t._v("\ns"),s("span",{pre:!0,attrs:{class:"token punctuation"}},[t._v(".")]),s("span",{pre:!0,attrs:{class:"token function"}},[t._v("AddReceive")]),s("span",{pre:!0,attrs:{class:"token punctuation"}},[t._v("(")]),t._v("signalChan"),s("span",{pre:!0,attrs:{class:"token punctuation"}},[t._v(",")]),t._v(" "),s("span",{pre:!0,attrs:{class:"token keyword"}},[t._v("func")]),s("span",{pre:!0,attrs:{class:"token punctuation"}},[t._v("(")]),t._v("c workflow"),s("span",{pre:!0,attrs:{class:"token punctuation"}},[t._v(".")]),t._v("Channel"),s("span",{pre:!0,attrs:{class:"token punctuation"}},[t._v(",")]),t._v(" more "),s("span",{pre:!0,attrs:{class:"token builtin"}},[t._v("bool")]),s("span",{pre:!0,attrs:{class:"token punctuation"}},[t._v(")")]),t._v(" "),s("span",{pre:!0,attrs:{class:"token punctuation"}},[t._v("{")]),t._v("\n    c"),s("span",{pre:!0,attrs:{class:"token punctuation"}},[t._v(".")]),s("span",{pre:!0,attrs:{class:"token function"}},[t._v("Receive")]),s("span",{pre:!0,attrs:{class:"token punctuation"}},[t._v("(")]),t._v("ctx"),s("span",{pre:!0,attrs:{class:"token punctuation"}},[t._v(",")]),t._v(" "),s("span",{pre:!0,attrs:{class:"token operator"}},[t._v("&")]),t._v("signalVal"),s("span",{pre:!0,attrs:{class:"token punctuation"}},[t._v(")")]),t._v("\n    workflow"),s("span",{pre:!0,attrs:{class:"token punctuation"}},[t._v(".")]),s("span",{pre:!0,attrs:{class:"token function"}},[t._v("GetLogger")]),s("span",{pre:!0,attrs:{class:"token punctuation"}},[t._v("(")]),t._v("ctx"),s("span",{pre:!0,attrs:{class:"token punctuation"}},[t._v(")")]),s("span",{pre:!0,attrs:{class:"token punctuation"}},[t._v(".")]),s("span",{pre:!0,attrs:{class:"token function"}},[t._v("Info")]),s("span",{pre:!0,attrs:{class:"token punctuation"}},[t._v("(")]),s("span",{pre:!0,attrs:{class:"token string"}},[t._v('"Received signal!"')]),s("span",{pre:!0,attrs:{class:"token punctuation"}},[t._v(",")]),t._v(" zap"),s("span",{pre:!0,attrs:{class:"token punctuation"}},[t._v(".")]),s("span",{pre:!0,attrs:{class:"token function"}},[t._v("String")]),s("span",{pre:!0,attrs:{class:"token punctuation"}},[t._v("(")]),s("span",{pre:!0,attrs:{class:"token string"}},[t._v('"signal"')]),s("span",{pre:!0,attrs:{class:"token punctuation"}},[t._v(",")]),t._v(" signalName"),s("span",{pre:!0,attrs:{class:"token punctuation"}},[t._v(")")]),s("span",{pre:!0,attrs:{class:"token punctuation"}},[t._v(",")]),t._v(" zap"),s("span",{pre:!0,attrs:{class:"token punctuation"}},[t._v(".")]),s("span",{pre:!0,attrs:{class:"token function"}},[t._v("String")]),s("span",{pre:!0,attrs:{class:"token punctuation"}},[t._v("(")]),s("span",{pre:!0,attrs:{class:"token string"}},[t._v('"value"')]),s("span",{pre:!0,attrs:{class:"token punctuation"}},[t._v(",")]),t._v(" signalVal"),s("span",{pre:!0,attrs:{class:"token punctuation"}},[t._v(")")]),s("span",{pre:!0,attrs:{class:"token punctuation"}},[t._v(")")]),t._v("\n"),s("span",{pre:!0,attrs:{class:"token punctuation"}},[t._v("}")]),s("span",{pre:!0,attrs:{class:"token punctuation"}},[t._v(")")]),t._v("\ns"),s("span",{pre:!0,attrs:{class:"token punctuation"}},[t._v(".")]),s("span",{pre:!0,attrs:{class:"token function"}},[t._v("Select")]),s("span",{pre:!0,attrs:{class:"token punctuation"}},[t._v("(")]),t._v("ctx"),s("span",{pre:!0,attrs:{class:"token punctuation"}},[t._v(")")]),t._v("\n\n"),s("span",{pre:!0,attrs:{class:"token keyword"}},[t._v("if")]),t._v(" "),s("span",{pre:!0,attrs:{class:"token function"}},[t._v("len")]),s("span",{pre:!0,attrs:{class:"token punctuation"}},[t._v("(")]),t._v("signalVal"),s("span",{pre:!0,attrs:{class:"token punctuation"}},[t._v(")")]),t._v(" "),s("span",{pre:!0,attrs:{class:"token operator"}},[t._v(">")]),t._v(" "),s("span",{pre:!0,attrs:{class:"token number"}},[t._v("0")]),t._v(" "),s("span",{pre:!0,attrs:{class:"token operator"}},[t._v("&&")]),t._v(" signalVal "),s("span",{pre:!0,attrs:{class:"token operator"}},[t._v("!=")]),t._v(" "),s("span",{pre:!0,attrs:{class:"token string"}},[t._v('"SOME_VALUE"')]),t._v(" "),s("span",{pre:!0,attrs:{class:"token punctuation"}},[t._v("{")]),t._v("\n    "),s("span",{pre:!0,attrs:{class:"token keyword"}},[t._v("return")]),t._v(" errors"),s("span",{pre:!0,attrs:{class:"token punctuation"}},[t._v(".")]),s("span",{pre:!0,attrs:{class:"token function"}},[t._v("New")]),s("span",{pre:!0,attrs:{class:"token punctuation"}},[t._v("(")]),s("span",{pre:!0,attrs:{class:"token string"}},[t._v('"signalVal"')]),s("span",{pre:!0,attrs:{class:"token punctuation"}},[t._v(")")]),t._v("\n"),s("span",{pre:!0,attrs:{class:"token punctuation"}},[t._v("}")]),t._v("\n")])])]),s("p",[t._v("In the example above, the "),s("Term",{attrs:{term:"workflow"}}),t._v(" code uses "),s("strong",[t._v("workflow.GetSignalChannel")]),t._v(" to open a\n"),s("strong",[t._v("workflow.Channel")]),t._v(" for the named "),s("Term",{attrs:{term:"signal"}}),t._v(". We then use a "),s("strong",[t._v("workflow.Selector")]),t._v(" to wait on this\nchannel and process the payload received with the "),s("Term",{attrs:{term:"signal"}}),t._v(".")],1),t._v(" "),s("h2",{attrs:{id:"signalwithstart"}},[s("a",{staticClass:"header-anchor",attrs:{href:"#signalwithstart"}},[t._v("#")]),t._v(" SignalWithStart")]),t._v(" "),s("p",[t._v("You may not know if a "),s("Term",{attrs:{term:"workflow"}}),t._v(" is running and can accept a "),s("Term",{attrs:{term:"signal"}}),t._v(". The\n"),s("a",{attrs:{href:"https://godoc.org/go.uber.org/cadence/client#Client",target:"_blank",rel:"noopener noreferrer"}},[t._v("client.SignalWithStartWorkflow"),s("OutboundLink")],1),t._v(" API\nallows you to send a "),s("Term",{attrs:{term:"signal"}}),t._v(" to the current "),s("Term",{attrs:{term:"workflow"}}),t._v(" instance if one exists or to create a new\nrun and then send the "),s("Term",{attrs:{term:"signal"}}),t._v(". "),s("code",[t._v("SignalWithStartWorkflow")]),t._v(" therefore doesn't take a "),s("Term",{attrs:{term:"run_ID"}}),t._v(" as a\nparameter.")],1)])}),[],!1,null,null,null);a.default=e.exports}}]);