module github.com/paul-carlton/goutils/pkg/kubectl

go 1.15

require (
	github.com/go-logr/logr v0.4.0
	github.com/golang/mock v1.5.0
	github.com/paul-carlton/goutils/pkg/logging v0.1.5
	github.com/paul-carlton/goutils/pkg/mocks/kubectl v0.1.5
	github.com/paul-carlton/goutils/pkg/mocks/logr v0.1.5
	github.com/pkg/errors v0.9.1
)

replace (
	github.com/paul-carlton/goutils/pkg/kubectl => ../kubectl
	github.com/paul-carlton/goutils/pkg/mocks/kubectl => ../mocks/kubectl
	github.com/paul-carlton/goutils/pkg/mocks/logr => ../mocks/logr
)
