NAME=prism

SHELL = /bin/bash

.PHONY: all test test-ci

all: test

test: 
	go test ./...

test-ci: 
	go get -f -u github.com/jstemmer/go-junit-report
	go test -v ./... -race | tee >(go-junit-report -package-name $(NAME) > $$CIRCLE_TEST_REPORTS/golang.xml); test $${PIPESTATUS[0]} -eq 0

test-ci-with-combined-coverage:
	echo 'mode: count' > /home/ubuntu/profile.cov ; find . -type d -not -path '*/\.*' -printf '%P\0' | xargs -0 -i sh -c 'echo > /home/ubuntu/tmp.cov && go test -v -covermode=count -coverprofile=/home/ubuntu/tmp.cov github.com/geckoboard/prism/{} && tail -n +2 /home/ubuntu/tmp.cov >> /home/ubuntu/profile.cov'
