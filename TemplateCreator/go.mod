module gitcode.com/cloud-native/cloud-native/TemplateCreator

go 1.26.5

require (
	cloud.google.com/go/artifactregistry v1.20.0
	cloud.google.com/go/storage v1.59.2
	connectrpc.com/connect v1.18.1
	github.com/ClickHouse/clickhouse-go/v2 v2.46.0
	github.com/Merovius/nbd v0.0.0-20240812113926-fd65a54c9949
	github.com/aws/aws-sdk-go-v2 v1.41.6
	github.com/aws/aws-sdk-go-v2/config v1.32.12
	github.com/aws/aws-sdk-go-v2/credentials v1.19.12
	github.com/aws/aws-sdk-go-v2/feature/s3/manager v1.20.12
	github.com/aws/aws-sdk-go-v2/service/ecr v1.44.0
	github.com/aws/aws-sdk-go-v2/service/s3 v1.100.0
	github.com/bits-and-blooms/bitset v1.24.5
	github.com/bmatcuk/doublestar/v4 v4.9.1
	github.com/caarlos0/env/v11 v11.3.1
	github.com/containernetworking/plugins v1.9.0
	github.com/coreos/go-iptables v0.8.0
	github.com/dchest/uniuri v1.2.0
	github.com/distribution/reference v0.6.0
	github.com/dustin/go-humanize v1.0.1
	github.com/edsrzf/mmap-go v1.2.1-0.20241212181136-fad1cd13edbd
	github.com/firecracker-microvm/firecracker-go-sdk v1.0.0
	github.com/go-openapi/errors v0.22.7
	github.com/go-openapi/runtime v0.29.2
	github.com/go-openapi/strfmt v0.26.1
	github.com/go-openapi/swag v0.25.5
	github.com/go-openapi/validate v0.25.2
	github.com/google/go-containerregistry v0.21.7
	github.com/google/nftables v0.3.0
	github.com/google/uuid v1.6.0
	github.com/googleapis/gax-go/v2 v2.21.0
	github.com/grpc-ecosystem/go-grpc-middleware/v2 v2.3.2
	github.com/hashicorp/consul/api v1.32.1
	github.com/hashicorp/go-retryablehttp v0.7.8
	github.com/inetaf/tcpproxy v0.0.0-20250222171855-c4b9df066048
	github.com/jellydator/ttlcache/v3 v3.4.0
	github.com/launchdarkly/go-sdk-common/v3 v3.3.0
	github.com/launchdarkly/go-server-sdk/v7 v7.13.0
	github.com/ngrok/firewall_toolkit v0.0.18
	github.com/oapi-codegen/runtime v1.4.0
	github.com/orcaman/concurrent-map/v2 v2.0.1
	github.com/shirou/gopsutil/v4 v4.26.3
	github.com/tklauser/go-sysconf v0.3.16
	github.com/vishvananda/netlink v1.3.1
	github.com/vishvananda/netns v0.0.5
	go.opentelemetry.io/contrib/bridges/otelzap v0.14.0
	go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc v0.67.0
	go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp v0.68.0
	go.opentelemetry.io/otel v1.44.0
	go.opentelemetry.io/otel/exporters/otlp/otlplog/otlploggrpc v0.20.0
	go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc v1.44.0
	go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc v1.44.0
	go.opentelemetry.io/otel/log v0.20.0
	go.opentelemetry.io/otel/metric v1.44.0
	go.opentelemetry.io/otel/sdk v1.44.0
	go.opentelemetry.io/otel/sdk/log v0.20.0
	go.opentelemetry.io/otel/sdk/metric v1.44.0
	go.opentelemetry.io/otel/trace v1.44.0
	go.uber.org/zap v1.27.1
	golang.org/x/mod v0.37.0
	golang.org/x/oauth2 v0.36.0
	golang.org/x/sync v0.21.0
	golang.org/x/sys v0.46.0
	google.golang.org/api v0.276.0
	google.golang.org/grpc v1.81.1
	google.golang.org/protobuf v1.36.11
	k8s.io/utils v0.0.0-20251002143259-bc988d571ff4
)

require (
	github.com/klauspost/compress v1.18.6 // indirect
	github.com/klauspost/pgzip v1.2.6 // indirect
	github.com/moby/sys/capability v0.4.0 // indirect
	github.com/opencontainers/runtime-spec v1.2.1 // indirect
	github.com/pierrec/lz4/v4 v4.1.26 // indirect
	github.com/ulikunitz/xz v0.5.12 // indirect
	golang.org/x/net v0.56.0 // indirect
)

require (
	cel.dev/expr v0.25.1 // indirect
	cloud.google.com/go v0.123.0 // indirect
	cloud.google.com/go/auth v0.20.0 // indirect
	cloud.google.com/go/auth/oauth2adapt v0.2.8 // indirect
	cloud.google.com/go/compute/metadata v0.9.0 // indirect
	cloud.google.com/go/iam v1.5.3 // indirect
	cloud.google.com/go/longrunning v0.8.0 // indirect
	cloud.google.com/go/monitoring v1.24.3 // indirect
	github.com/ClickHouse/ch-go v0.71.0 // indirect
	github.com/DataDog/datadog-go/v5 v5.2.0 // indirect
	github.com/GoogleCloudPlatform/opentelemetry-operations-go/detectors/gcp v1.31.0 // indirect
	github.com/GoogleCloudPlatform/opentelemetry-operations-go/exporter/metric v0.54.0 // indirect
	github.com/GoogleCloudPlatform/opentelemetry-operations-go/internal/resourcemapping v0.54.0 // indirect
	github.com/Microsoft/go-winio v0.6.2 // indirect
	github.com/armon/go-metrics v0.4.1 // indirect
	github.com/aws/aws-sdk-go-v2/aws/protocol/eventstream v1.7.9 // indirect
	github.com/aws/aws-sdk-go-v2/feature/ec2/imds v1.18.20 // indirect
	github.com/aws/aws-sdk-go-v2/internal/configsources v1.4.22 // indirect
	github.com/aws/aws-sdk-go-v2/internal/endpoints/v2 v2.7.22 // indirect
	github.com/aws/aws-sdk-go-v2/internal/ini v1.8.6 // indirect
	github.com/aws/aws-sdk-go-v2/internal/v4a v1.4.23 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/accept-encoding v1.13.8 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/checksum v1.9.14 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/presigned-url v1.13.22 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/s3shared v1.19.22 // indirect
	github.com/aws/aws-sdk-go-v2/service/signin v1.0.8 // indirect
	github.com/aws/aws-sdk-go-v2/service/sso v1.30.13 // indirect
	github.com/aws/aws-sdk-go-v2/service/ssooidc v1.35.17 // indirect
	github.com/aws/aws-sdk-go-v2/service/sts v1.41.9 // indirect
	github.com/aws/smithy-go v1.25.0 // indirect
	github.com/cenkalti/backoff/v5 v5.0.3 // indirect
	github.com/cespare/xxhash/v2 v2.3.0 // indirect
	github.com/cncf/xds/go v0.0.0-20260202195803-dba9d589def2 // indirect
	github.com/containerd/errdefs v1.0.0 // indirect
	github.com/containerd/errdefs/pkg v0.3.0 // indirect
	github.com/containerd/fifo v1.1.0 // indirect
	github.com/containernetworking/cni v1.3.0 // indirect
	github.com/containers/storage v1.58.0
	github.com/docker/cli v29.5.3+incompatible // indirect
	github.com/docker/docker-credential-helpers v0.9.3 // indirect
	github.com/docker/go-connections v0.7.0 // indirect
	github.com/docker/go-units v0.5.0 // indirect
	github.com/ebitengine/purego v0.10.0 // indirect
	github.com/envoyproxy/go-control-plane/envoy v1.37.0 // indirect
	github.com/envoyproxy/protoc-gen-validate v1.3.3 // indirect
	github.com/fatih/color v1.18.0 // indirect
	github.com/felixge/httpsnoop v1.0.4 // indirect
	github.com/gaissmai/extnetip v0.3.3 // indirect
	github.com/go-faster/city v1.0.1 // indirect
	github.com/go-faster/errors v0.7.1 // indirect
	github.com/go-jose/go-jose/v4 v4.1.4 // indirect
	github.com/go-logr/logr v1.4.3 // indirect
	github.com/go-logr/stdr v1.2.2 // indirect
	github.com/go-ole/go-ole v1.2.6 // indirect
	github.com/go-openapi/analysis v0.24.3 // indirect
	github.com/go-openapi/jsonpointer v0.22.5 // indirect
	github.com/go-openapi/jsonreference v0.21.5 // indirect
	github.com/go-openapi/loads v0.23.3 // indirect
	github.com/go-openapi/spec v0.22.4 // indirect
	github.com/go-openapi/swag/cmdutils v0.25.5 // indirect
	github.com/go-openapi/swag/conv v0.25.5 // indirect
	github.com/go-openapi/swag/fileutils v0.25.5 // indirect
	github.com/go-openapi/swag/jsonname v0.25.5 // indirect
	github.com/go-openapi/swag/jsonutils v0.25.5 // indirect
	github.com/go-openapi/swag/loading v0.25.5 // indirect
	github.com/go-openapi/swag/mangling v0.25.5 // indirect
	github.com/go-openapi/swag/netutils v0.25.5 // indirect
	github.com/go-openapi/swag/stringutils v0.25.5 // indirect
	github.com/go-openapi/swag/typeutils v0.25.5 // indirect
	github.com/go-openapi/swag/yamlutils v0.25.5 // indirect
	github.com/go-viper/mapstructure/v2 v2.5.0 // indirect
	github.com/google/go-cmp v0.7.0 // indirect
	github.com/google/pprof v0.0.0-20260302011040-a15ffb7f9dcc // indirect
	github.com/google/s2a-go v0.1.9 // indirect
	github.com/googleapis/enterprise-certificate-proxy v0.3.14 // indirect
	github.com/gregjones/httpcache v0.0.0-20190611155906-901d90724c79 // indirect
	github.com/grpc-ecosystem/grpc-gateway/v2 v2.29.0 // indirect
	github.com/hashicorp/errwrap v1.1.0 // indirect
	github.com/hashicorp/go-cleanhttp v0.5.2 // indirect
	github.com/hashicorp/go-hclog v1.6.3 // indirect
	github.com/hashicorp/go-immutable-radix v1.3.1 // indirect
	github.com/hashicorp/go-metrics v0.5.4 // indirect
	github.com/hashicorp/go-msgpack/v2 v2.1.5 // indirect
	github.com/hashicorp/go-multierror v1.1.1 // indirect
	github.com/hashicorp/go-rootcerts v1.0.2 // indirect
	github.com/hashicorp/golang-lru v1.0.2 // indirect
	github.com/hashicorp/memberlist v0.5.4 // indirect
	github.com/hashicorp/serf v0.10.2 // indirect
	github.com/josharian/intern v1.0.0 // indirect
	github.com/launchdarkly/ccache v1.1.0 // indirect
	github.com/launchdarkly/eventsource v1.10.0 // indirect
	github.com/launchdarkly/go-jsonstream/v3 v3.1.0 // indirect
	github.com/launchdarkly/go-sdk-events/v3 v3.5.0 // indirect
	github.com/launchdarkly/go-semver v1.0.3 // indirect
	github.com/launchdarkly/go-server-sdk-evaluation/v3 v3.0.1 // indirect
	github.com/lufia/plan9stats v0.0.0-20211012122336-39d0f177ccd0 // indirect
	github.com/mailru/easyjson v0.9.1 // indirect
	github.com/mattn/go-colorable v0.1.14 // indirect
	github.com/mattn/go-isatty v0.0.22 // indirect
	github.com/mdlayher/genetlink v1.3.2 // indirect
	github.com/mdlayher/netlink v1.7.3-0.20250113171957-fbb4dce95f42 // indirect
	github.com/mdlayher/socket v0.5.1 // indirect
	github.com/miekg/dns v1.1.72 // indirect
	github.com/mitchellh/go-homedir v1.1.0 // indirect
	github.com/mitchellh/mapstructure v1.5.1-0.20231216201459-8508981c8b6c // indirect
	github.com/moby/docker-image-spec v1.3.1 // indirect
	github.com/moby/moby/api v1.55.0 // indirect
	github.com/moby/moby/client v0.5.0 // indirect
	github.com/moby/sys/mountinfo v0.7.2 // indirect
	github.com/moby/sys/user v0.4.0 // indirect
	github.com/oklog/ulid/v2 v2.1.1 // indirect
	github.com/onsi/ginkgo/v2 v2.27.2 // indirect
	github.com/onsi/gomega v1.38.2 // indirect
	github.com/opencontainers/go-digest v1.0.0 // indirect
	github.com/opencontainers/image-spec v1.1.1 // indirect
	github.com/patrickmn/go-cache v2.1.0+incompatible // indirect
	github.com/paulmach/orb v0.13.0 // indirect
	github.com/planetscale/vtprotobuf v0.6.1-0.20240319094008-0393e58bdf10 // indirect
	github.com/power-devops/perfstat v0.0.0-20240221224432-82ca36839d55 // indirect
	github.com/rogpeppe/go-internal v1.15.0 // indirect
	github.com/rs/zerolog v1.34.0 // indirect
	github.com/segmentio/asm v1.2.1 // indirect
	github.com/shopspring/decimal v1.4.0 // indirect
	github.com/sirupsen/logrus v1.9.4 // indirect
	github.com/spiffe/go-spiffe/v2 v2.6.0 // indirect
	github.com/stretchr/objx v0.5.3 // indirect
	github.com/tklauser/numcpus v0.11.0 // indirect
	github.com/yusufpapurcu/wmi v1.2.4 // indirect
	go.opentelemetry.io/auto/sdk v1.2.1 // indirect
	go.opentelemetry.io/contrib/detectors/gcp v1.42.0 // indirect
	go.opentelemetry.io/otel/exporters/otlp/otlptrace v1.44.0 // indirect
	go.opentelemetry.io/otel/exporters/stdout/stdoutmetric v1.39.0 // indirect
	go.opentelemetry.io/proto/otlp v1.10.0 // indirect
	go.uber.org/multierr v1.11.0 // indirect
	go.yaml.in/yaml/v3 v3.0.4 // indirect
	golang.org/x/crypto v0.53.0 // indirect
	golang.org/x/exp v0.0.0-20260410095643-746e56fc9e2f // indirect
	golang.org/x/text v0.38.0 // indirect
	golang.org/x/time v0.15.0 // indirect
	google.golang.org/genproto v0.0.0-20260319201613-d00831a3d3e7 // indirect
	google.golang.org/genproto/googleapis/api v0.0.0-20260526163538-3dc84a4a5aaa // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20260526163538-3dc84a4a5aaa // indirect
)

replace github.com/willscott/go-nfs v0.0.3 => github.com/e2b-dev/go-nfs v0.0.0-20260318224420-f59b77ca8555
