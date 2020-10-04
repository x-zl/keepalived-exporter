package collector

import (
	"bytes"
	"context"
	"io/ioutil"

	"github.com/docker/docker/api/types"
	"github.com/sirupsen/logrus"
)

func (k *KeepalivedCollector) dockerExecCmd(cmd []string, container string) (*bytes.Buffer, error) {
	rst, err := k.dockerCli.ContainerExecCreate(context.Background(), container, types.ExecConfig{
		AttachStdout: true,
		AttachStderr: true,
		Cmd:          cmd,
	})
	if err != nil {
		logrus.WithError(err).WithField("CMD", cmd).Error("Error creating exec container")
		return nil, err
	}

	response, err := k.dockerCli.ContainerExecAttach(context.Background(), rst.ID, types.ExecConfig{})
	if err != nil {
		logrus.WithError(err).WithField("CMD", cmd).Error("Error attaching a connection to an exec process")
		return nil, err
	}
	defer response.Close()

	data, err := ioutil.ReadAll(response.Reader)
	if err != nil {
		logrus.WithError(err).WithField("CMD", cmd).Error("Error reading response from docker exec command")
		return nil, err
	}

	return bytes.NewBuffer(data), nil
}

func (k *KeepalivedCollector) dockerKillContainer(container, signal string) error {
	return k.dockerCli.ContainerKill(context.Background(), container, signal)
}
