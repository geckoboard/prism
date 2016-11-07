NAME=prism

SHELL = /bin/bash

.PHONY: all test test-ci

all: test

test: 
	go test ./...

test-ci: 
	go get -f -u github.com/jstemmer/go-junit-report
	go test -v ./... -race | tee >(go-junit-report -package-name $(NAME) > $$CIRCLE_TEST_REPORTS/golang.xml); test $${PIPESTATUS[0]} -eq 0
