#!/bin/bash

for i in {1..100}
do
  curl -X POST -d "msg=Foo${i}" http://127.0.0.1:3004/post
done