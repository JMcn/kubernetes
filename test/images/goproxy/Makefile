# Copyright 2016 The Kubernetes Authors All rights reserved.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

TAG = 0.1
PREFIX = index.alauda.cn/googlecontainer

all: push

goproxy: goproxy.go
	CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -ldflags '-w' ./goproxy.go

image: goproxy
	docker build -t $(PREFIX)/goproxy:$(TAG) .

push: image
	gcloud docker push $(PREFIX)/goproxy:$(TAG)

clean:
	rm -f goproxy
