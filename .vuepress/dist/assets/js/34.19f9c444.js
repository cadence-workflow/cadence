(window.webpackJsonp=window.webpackJsonp||[]).push([[34],{405:function(a,e,s){"use strict";s.r(e);var n=s(8),t=Object(n.a)({},(function(){var a=this,e=a.$createElement,s=a._self._c||e;return s("ContentSlotsDistributor",{attrs:{"slot-key":a.$parent.slotKey}},[s("h1",{attrs:{id:"getting-started"}},[s("a",{staticClass:"header-anchor",attrs:{href:"#getting-started"}},[a._v("#")]),a._v(" Getting started")]),a._v(" "),s("h2",{attrs:{id:"installing-cadence-service-and-ui-on-a-mac"}},[s("a",{staticClass:"header-anchor",attrs:{href:"#installing-cadence-service-and-ui-on-a-mac"}},[a._v("#")]),a._v(" Installing Cadence Service and UI on a Mac")]),a._v(" "),s("figure",{staticClass:"video-container"},[s("iframe",{attrs:{src:"https://www.youtube.com/embed/aLyRyNe5Ls0",frameborder:"0",height:"315",allowfullscreen:"",width:"560"}})]),a._v(" "),s("p",[a._v("Commands executed during the tutorial:")]),a._v(" "),s("div",{staticClass:"language-bash extra-class"},[s("pre",{pre:!0,attrs:{class:"language-bash"}},[s("code",[a._v("docker-compose up\n\ndocker run --rm ubercadence/cli:master --address host.docker.internal:7933 --domain samples-domain domain register\n\ndocker run --rm ubercadence/cli:master --address host.docker.internal:7933 --domain samples-domain domain describe\n\n"),s("span",{pre:!0,attrs:{class:"token builtin class-name"}},[a._v("alias")]),a._v(" "),s("span",{pre:!0,attrs:{class:"token assign-left variable"}},[a._v("cadence")]),s("span",{pre:!0,attrs:{class:"token operator"}},[a._v("=")]),s("span",{pre:!0,attrs:{class:"token string"}},[a._v('"docker run --rm ubercadence/cli:master --address host.docker.internal:7933"')]),a._v("\n\ncadence --domain samples-domain domain desc\n\ncadence "),s("span",{pre:!0,attrs:{class:"token builtin class-name"}},[a._v("help")]),a._v("\n\ncadence workflow "),s("span",{pre:!0,attrs:{class:"token builtin class-name"}},[a._v("help")]),a._v("\n\ncadence --domain samples-domain workflow list\n\ncadence --domain samples-domain workflow "),s("span",{pre:!0,attrs:{class:"token builtin class-name"}},[a._v("help")]),a._v(" start\n\ncadence --domain samples-domain workflow start -wt "),s("span",{pre:!0,attrs:{class:"token builtin class-name"}},[a._v("test")]),a._v(" -tl "),s("span",{pre:!0,attrs:{class:"token builtin class-name"}},[a._v("test")]),a._v(" -et "),s("span",{pre:!0,attrs:{class:"token number"}},[a._v("300")]),a._v("\n\ncadence --domain samples-domain workflow list -op\n\ncadence --domain samples-domain workflow terminate -wid "),s("span",{pre:!0,attrs:{class:"token operator"}},[a._v("<")]),a._v("workflowID"),s("span",{pre:!0,attrs:{class:"token operator"}},[a._v(">")]),a._v("\n\n")])])])])}),[],!1,null,null,null);e.default=t.exports}}]);