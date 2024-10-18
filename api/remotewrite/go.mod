module github.com/prometheus/client_golang/api/remotewrite

go 1.22.6

require (
	github.com/efficientgo/core v1.0.0-rc.3 // TODO(bwplotka): Remove, vendor this instead or reimplement.
	github.com/google/go-cmp v0.6.0
	github.com/klauspost/compress v1.17.9
	github.com/planetscale/vtprotobuf v0.6.0 // TODO(bwplotka): Remove, use interface instead to allow this for advanced users for efficiency.
	github.com/prometheus/common v0.60.0
	github.com/stretchr/testify v1.9.0
	google.golang.org/protobuf v1.34.2
)

require (
	github.com/davecgh/go-spew v1.1.2-0.20180830191138-d8f796af33cc // indirect
	github.com/kr/pretty v0.3.1 // indirect
	github.com/pmezard/go-difflib v1.0.1-0.20181226105442-5d4384ee4fb2 // indirect
	github.com/prometheus/client_model v0.6.1 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)
