#!/bin/bash

protoc -I. pkg/playsource/playsource_service.proto --go_out=plugins=grpc:.

