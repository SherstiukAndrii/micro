#!/bin/bash

for i in {1..100}
do
  curl -X POST -d "msg=Foo${i}" http://127.0.0.1:46681/post
done

# sleep 30

# curl http://127.0.0.1:46681/get

# for service in $(curl -s http://localhost:8500/v1/agent/services | jq -r 'keys[]'); do
#   curl -s -X PUT http://localhost:8500/v1/agent/service/deregister/$service
# done