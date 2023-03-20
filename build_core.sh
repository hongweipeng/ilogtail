#! /bin/bash
set -e

DOCKER_IMAGE="sls-opensource-registry.cn-shanghai.cr.aliyuncs.com/ilogtail-community-edition/ilogtail-build-linux"

mkdir -p build/core
mkdir -p output

docker run --rm -u `id -u $USER` -v $(pwd):/work -w /work/build/core $DOCKER_IMAGE bash -c "if [[ ! -f Makefile ]]; then cmake ../../core; fi; make -j2"


cp -a ./build/core/ilogtail ./output
cp -a ./build/core/plugin/libPluginAdapter.so ./output

docker run --rm -u `id -u $USER` -v $(pwd):/work -w /work $DOCKER_IMAGE bash -c "make plugin_local"
