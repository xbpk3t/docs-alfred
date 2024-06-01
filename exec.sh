#!/bin/bash

go run main.go f qs --config=qs.yml

go run main.go f gh --config=gh.yml

# f gh --config=gh.yml
# sync --config=gh.yml

go run main.go md qs --config qs.yml --target qs.md

alfred_workflow_bundleid=com.hapihacking.pwgen alfred_workflow_cache=./testenv/cache alfred_workflow_data=./testenv/cache alfred_workflow_name=docs-alfred alfred_workflow_version=2.0.0 go run main.go md qs --config qs.yml --target qs.md



