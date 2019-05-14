#!/bin/bash -e

rm -rf ./cov
mkdir cov
go test -covermode=atomic -coverprofile=./cov/conf.out ./server/conf
go test -covermode=atomic -coverprofile=./cov/core.out ./server/core
go test -covermode=atomic -coverprofile=./cov/logging.out ./server/logging

gocovmerge ./cov/*.out > ./coverage.out
rm -rf ./cov

# If we have an arg, assume travis run and push to coveralls. Otherwise launch browser results
if [[ -n $1 ]]; then
    $HOME/gopath/bin/goveralls -coverprofile=coverage.out -service travis-ci
    rm -rf ./coverage.out
else
    go tool cover -html=coverage.out
fi