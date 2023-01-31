// Copyright 2021 iLogtail Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

//go:build linux || windows
// +build linux windows

package pluginmanager

import (
	"context"
	"testing"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/stretchr/testify/suite"

	"github.com/alibaba/ilogtail"
	"github.com/alibaba/ilogtail/helper"
	"github.com/alibaba/ilogtail/pkg/flags"
	"github.com/alibaba/ilogtail/pkg/logger"
	"github.com/alibaba/ilogtail/pkg/protocol"
	"github.com/alibaba/ilogtail/pkg/util"
	"github.com/alibaba/ilogtail/plugins/flusher/checker"
	_ "github.com/alibaba/ilogtail/plugins/flusher/sls"
	_ "github.com/alibaba/ilogtail/plugins/input/docker/stdout"
)

func TestContainerConfig(t *testing.T) {
	suite.Run(t, new(containerConfigTestSuite))
}

func (s *containerConfigTestSuite) TestRefreshEnvAndLabel() {
	s.NoError(loadMockConfig(), "got err when logad config")
	refreshEnvAndLabel()
	s.Equal(1, len(LogtailConfig))
	s.Equal(1, len(envSet))
	s.Equal(1, len(labelSet))
}

func (s *containerConfigTestSuite) TestCompareEnvAndLabel() {
	envSet = make(map[string]struct{})
	labelSet = make(map[string]struct{})

	s.NoError(loadMockConfig(), "got err when logad config")

	envSet["testEnv1"] = struct{}{}
	labelSet["testLebel1"] = struct{}{}

	diffEnvSet, diffLabelSet := compareEnvAndLabel()
	s.Equal(1, len(diffEnvSet))
	s.Equal(1, len(diffLabelSet))
	s.Equal(2, len(envSet))
	s.Equal(2, len(labelSet))
}

func (s *containerConfigTestSuite) TestCompareEnvAndLabelAndRecordContainer() {
	envSet = make(map[string]struct{})
	labelSet = make(map[string]struct{})

	s.NoError(loadMockConfig(), "got err when logad config")

	envSet["testEnv1"] = struct{}{}
	labelSet["testLebel1"] = struct{}{}

	envList := []string{0: "test=111"}
	info := mockDockerInfoDetail("testConfig", envList)
	cMap := helper.GetContainerMap()
	cMap["test"] = info

	compareEnvAndLabelAndRecordContainer()
	s.Equal(1, len(util.AddedContainers))
	util.AddedContainers = util.AddedContainers[:0]
}

func (s *containerConfigTestSuite) TestRecordContainers() {
	info := mockDockerInfoDetail("testConfig", []string{0: "test=111"})
	cMap := helper.GetContainerMap()
	cMap["test"] = info

	containerIDs := make(map[string]struct{})
	containerIDs["test"] = struct{}{}
	recordContainers(containerIDs)
	s.Equal(1, len(util.AddedContainers))
	util.AddedContainers = util.AddedContainers[:0]
}

type containerConfigTestSuite struct {
	suite.Suite
}

func (s *containerConfigTestSuite) BeforeTest(suiteName, testName string) {
	logger.Infof(context.Background(), "========== %s %s test start ========================", suiteName, testName)
}

func (s *containerConfigTestSuite) AfterTest(suiteName, testName string) {
	logger.Infof(context.Background(), "========== %s %s test end =======================", suiteName, testName)

}

func mockDockerInfoDetail(containerName string, envList []string) *helper.DockerInfoDetail {
	dockerInfo := types.ContainerJSON{
		ContainerJSONBase: &types.ContainerJSONBase{
			Name: containerName,
			ID:   "test",
		},
	}
	dockerInfo.Config = &container.Config{}
	dockerInfo.Config.Env = envList
	return helper.CreateContainerInfoDetail(dockerInfo, *flags.LogConfigPrefix, false)
}

// project, logstore, config, configJsonStr
func loadMockConfig() error {
	project := "test_prj"
	logstore := "test_logstore"
	configName := "test_config"

	configStr := `
	{
		"inputs": [{
			"detail": {
				"Stderr": true,
				"IncludeLabel": {
					"app": "^.*$"
				},
				"IncludeEnv": {
					"test": "^.*$"
				},
				"Stdout": true
			},
			"type": "service_docker_stdout"
		}],
		"global": {
			"AlwaysOnline": true,
			"type": "dockerStdout"
		}
	}`
	return LoadLogstoreConfig(project, logstore, configName, 666, configStr)
}

func (s *containerConfigTestSuite) TestLargeCountLog() {
	configStr := `
	{
		"global": {
			"InputIntervalMs" :  30000,
			"AggregatIntervalMs": 1000,
			"FlushIntervalMs": 1000,
			"DefaultLogQueueSize": 4,
			"DefaultLogGroupQueueSize": 4,
			"Tags" : {
				"base_version" : "0.1.0",
				"logtail_version" : "0.16.19"
			}
		},
		"inputs" : [
			{
				"type" : "metric_container",
				"detail" : null
			}
		],
		"flushers": [
			{
				"type": "flusher_checker"
			}
		]
	}`
	nowTime := (uint32)(time.Now().Unix())
	ContainerConfig, err := loadBuiltinConfig("container", "sls-test", "logtail_containers", "logtail_containers", configStr)
	s.NoError(err)
	ContainerConfig.Start()
	loggroup := &protocol.LogGroup{}
	for i := 1; i <= 100000; i++ {
		log := &protocol.Log{}
		log.Contents = append(log.Contents, &protocol.Log_Content{Key: "test", Value: "123"})
		log.Time = nowTime
		loggroup.Logs = append(loggroup.Logs, log)
	}

	for _, log := range loggroup.Logs {
		ContainerConfig.PluginRunner.ReceiveRawLog(&ilogtail.LogWithContext{Log: log})
	}
	s.Equal(1, len(GetConfigFluhsers(ContainerConfig.PluginRunner)))
	time.Sleep(time.Millisecond * time.Duration(1500))
	c, ok := GetConfigFluhsers(ContainerConfig.PluginRunner)[0].(*checker.FlusherChecker)
	s.True(ok)
	s.Equal(100000, c.GetLogCount())
}