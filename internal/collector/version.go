package collector

import (
	"bytes"
	"errors"
	"os/exec"
	"path"
	"strings"

	"github.com/cafebazaar/keepalived-exporter/internal/utils"
	"github.com/hashicorp/go-version"
	"github.com/sirupsen/logrus"
)

func (k *KeepalivedCollector) getKeepalivedVersion() (*version.Version, error) {
	getVersionCmd := []string{"-v"}
	var outputCmd *bytes.Buffer
	var err error
	if k.containerName != "" {
		outputCmd, err = utils.DockerExecCmd(append([]string{"keepalived"}, getVersionCmd...), k.containerName)
		if err != nil {
			return nil, err
		}
	} else if k.endpoint != nil {
		u := *k.endpoint
		u.Path = path.Join(k.endpoint.Path, "version")
		outputCmd, err = utils.EndpointExec(&u)
		if err != nil {
			return nil, err
		}
	} else {
		cmd := exec.Command("keepalived", getVersionCmd...)
		var stdout, stderr bytes.Buffer
		cmd.Stdout = &stdout
		cmd.Stderr = &stderr
		err := cmd.Run()
		if err != nil {
			logrus.WithFields(logrus.Fields{"stderr": stderr.String(), "stdout": stdout.String()}).WithError(err).Error("Error getting keepalived version")
			return nil, errors.New("Error getting keepalived version")
		}
		outputCmd = &stderr
	}

	// version is always at first line
	firstLine, err := outputCmd.ReadString('\n')
	if err != nil {
		logrus.WithField("output", outputCmd.String()).WithError(err).Error("Failed to parse keepalived version output")
		return nil, errors.New("Failed to parse keepalived version output")
	}

	args := strings.Split(firstLine, " ")
	if len(args) < 2 {
		logrus.WithField("firstLine", firstLine).Error("Unknown keepalived version format")
		return nil, errors.New("Unknown keepalived version format")
	}

	keepalivedVersion, err := version.NewVersion(args[1][1:])
	if err != nil {
		logrus.WithField("version", args[1][1:]).WithError(err).Error("Failed to parse keepalived version")
		return nil, errors.New("Failed to parse keepalived version")
	}

	return keepalivedVersion, nil
}
