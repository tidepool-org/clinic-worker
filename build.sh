#!/bin/sh -eu

rm -rf dist
mkdir dist
go build -o dist/worker main.go
