NAME=prism

SHELL = /bin/bash

.PHONY: all test test-ci

all: test

test: 
	go test `go list ./... | grep -v "vendor"`

test-ci: 
	go get -f -u github.com/jstemmer/go-junit-report
	go test -v -race `go list ./... | grep -v "vendor"` | tee >(go-junit-report -package-name $(NAME) > $$CIRCLE_TEST_REPORTS/golang.xml); test $${PIPESTATUS[0]} -eq 0

test-ci-with-combined-coverage:
	echo 'mode: count' > /home/ubuntu/profile.cov ; go list ./... | grep -v "vendor" | xargs -i sh -c 'echo > /home/ubuntu/tmp.cov && go test -v -covermode=count -coverprofile=/home/ubuntu/tmp.cov {} && tail -n +2 /home/ubuntu/tmp.cov >> /home/ubuntu/profile.cov'
