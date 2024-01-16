module github.com/paul-carlton/goutils/pkg/mocks/kubectl

go 1.21

require (
	github.com/go-logr/logr v0.4.0
	github.com/golang/mock v1.5.0
	github.com/paul-carlton/goutils/pkg/kubectl v0.0.3
)

replace github.com/paul-carlton/goutils/pkg/kubectl => ../../kubectl
