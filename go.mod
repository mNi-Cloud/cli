module github.com/mNi-Cloud/cli

go 1.22.7

toolchain go1.22.8

require (
	github.com/coreos/go-oidc v2.2.1+incompatible
	github.com/google/uuid v1.6.0
	github.com/gorilla/websocket v1.5.0
	github.com/jedib0t/go-pretty/v6 v6.5.9
	github.com/labstack/gommon v0.4.2
	github.com/mNi-Cloud/backend/auth v0.0.0-20240905053311-72241b9763a7
	github.com/mNi-Cloud/backend/bs v0.0.0-20241029042900-89981571e097
	github.com/mNi-Cloud/backend/common v0.0.0-20241029042900-89981571e097
	github.com/mNi-Cloud/backend/ctr v0.0.0-20241029042900-89981571e097
	github.com/mNi-Cloud/backend/vm v0.0.0-20241029042900-89981571e097
	github.com/mNi-Cloud/backend/vpc v0.0.0-20241029042900-89981571e097
	github.com/urfave/cli/v2 v2.27.4
	golang.org/x/oauth2 v0.23.0
	golang.org/x/term v0.24.0
)

require (
	github.com/apapsch/go-jsonmerge/v2 v2.0.0 // indirect
	github.com/cpuguy83/go-md2man/v2 v2.0.4 // indirect
	github.com/fxamacker/cbor/v2 v2.7.0 // indirect
	github.com/go-logr/logr v1.4.2 // indirect
	github.com/gogo/protobuf v1.3.2 // indirect
	github.com/google/gofuzz v1.2.0 // indirect
	github.com/iancoleman/strcase v0.3.0 // indirect
	github.com/json-iterator/go v1.1.12 // indirect
	github.com/labstack/echo/v4 v4.12.0 // indirect
	github.com/mattn/go-colorable v0.1.13 // indirect
	github.com/mattn/go-isatty v0.0.20 // indirect
	github.com/mattn/go-runewidth v0.0.15 // indirect
	github.com/modern-go/concurrent v0.0.0-20180306012644-bacd9c7ef1dd // indirect
	github.com/modern-go/reflect2 v1.0.2 // indirect
	github.com/oapi-codegen/runtime v1.1.1 // indirect
	github.com/pquerna/cachecontrol v0.2.0 // indirect
	github.com/rivo/uniseg v0.2.0 // indirect
	github.com/russross/blackfriday/v2 v2.1.0 // indirect
	github.com/valyala/bytebufferpool v1.0.0 // indirect
	github.com/valyala/fasttemplate v1.2.2 // indirect
	github.com/x448/float16 v0.8.4 // indirect
	github.com/xrash/smetrics v0.0.0-20240521201337-686a1a2994c1 // indirect
	golang.org/x/crypto v0.27.0 // indirect
	golang.org/x/exp v0.0.0-20240909161429-701f63a606c0 // indirect
	golang.org/x/net v0.29.0 // indirect
	golang.org/x/sys v0.25.0 // indirect
	golang.org/x/text v0.18.0 // indirect
	gopkg.in/inf.v0 v0.9.1 // indirect
	gopkg.in/square/go-jose.v2 v2.6.0 // indirect
	gopkg.in/yaml.v2 v2.4.0 // indirect
	k8s.io/apimachinery v0.31.1 // indirect
	k8s.io/klog/v2 v2.130.1 // indirect
	k8s.io/utils v0.0.0-20240902221715-702e33fdd3c3 // indirect
	sigs.k8s.io/json v0.0.0-20221116044647-bc3834ca7abd // indirect
	sigs.k8s.io/structured-merge-diff/v4 v4.4.1 // indirect
)

replace (
	github.com/openshift/api => github.com/openshift/api v0.0.0-20191219222812-2987a591a72c
	github.com/openshift/client-go => github.com/openshift/client-go v0.0.0-20221107163225-3335a34a1d24
	k8s.io/client-go => k8s.io/client-go v0.30.0
	k8s.io/dynamic-resource-allocation => k8s.io/dynamic-resource-allocation v0.27.10
	k8s.io/kube-openapi => k8s.io/kube-openapi v0.0.0-20240228011516-70dd3763d340
	k8s.io/kube-scheduler => k8s.io/kube-scheduler v0.27.10
	k8s.io/legacy-cloud-providers => k8s.io/legacy-cloud-providers v0.27.10
	k8s.io/mount-utils => k8s.io/mount-utils v0.27.10
	kubevirt.io/api => kubevirt.io/api v1.3.0-rc.1
)
