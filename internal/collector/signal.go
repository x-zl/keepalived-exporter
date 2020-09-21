package collector

import (
	"bytes"
	"io/ioutil"
	"os"
	"os/exec"
	"path"
	"regexp"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/cafebazaar/keepalived-exporter/internal/utils"
	"github.com/hashicorp/go-version"
	"github.com/sirupsen/logrus"
)

var sigNumSupportedVersion = version.Must(version.NewVersion("1.3.8"))

func (k *KeepalivedCollector) isSigNumSupport() bool {
	keepalivedVersion, err := k.getKeepalivedVersion()
	if err != nil {
		// keep backward compatibility and assuming it's the latest one on version detection failure
		return true
	}
	return keepalivedVersion.GreaterThanOrEqual(sigNumSupportedVersion)
}

func (k *KeepalivedCollector) sigNum(sig string) int {
	if !k.isSigNumSupport() {
		switch sig {
		case "DATA":
			return 10
		case "STATS":
			return 12
		default:
			logrus.WithField("signal", sig).Fatal("Unsupported signal for your keepalived")
		}
	}

	sigNumCmd := []string{"--signum", sig}
	var outputCmd *bytes.Buffer
	var err error

	if k.containerName != "" {
		outputCmd, err = utils.DockerExecCmd(append([]string{"keepalived"}, sigNumCmd...), k.containerName)
		if err != nil {
			logrus.WithFields(logrus.Fields{"signal": sig, "container": k.containerName}).WithError(err).Fatal("Error getting signum")
		}
	} else if k.endpoint != nil {
		u := *k.endpoint
		u.Path = path.Join(u.Path, "signal/num")
		queryString := u.Query()
		queryString.Set("signal", sig)
		u.RawQuery = queryString.Encode()
		outputCmd, err = utils.EndpointExec(&u)
		if err != nil {
			logrus.WithField("endpoint", k.endpoint.String()).WithError(err).Fatal("Error getting signum")
		}
	} else {
		cmd := exec.Command("keepalived", sigNumCmd...)
		var stderr bytes.Buffer
		cmd.Stdout = outputCmd
		cmd.Stderr = &stderr
		err := cmd.Run()
		if err != nil {
			logrus.WithFields(logrus.Fields{"signal": sig, "stderr": stderr.String()}).WithError(err).Fatal("Error getting signum")
		}
	}

	reg, err := regexp.Compile("[^0-9]+")
	if err != nil {
		logrus.WithError(err).Fatal("Unexcpected error occures in creating regex")
	}

	strSigNum := reg.ReplaceAllString(outputCmd.String(), "")
	signum, err := strconv.Atoi(strSigNum)
	if err != nil {
		logrus.WithFields(logrus.Fields{"signal": sig, "signum": outputCmd}).WithError(err).Fatal("Error unmarshalling signum result")
	}

	return signum
}

func (k *KeepalivedCollector) signal(signal int) error {
	if k.containerName != "" {
		return utils.DockerKillContainer(k.containerName, strconv.Itoa((signal)))
	} else if k.endpoint != nil {
		u := *k.endpoint
		u.Path = path.Join(u.Path, "signal")
		queryString := u.Query()
		queryString.Set("signal", strconv.Itoa(signal))
		u.RawQuery = queryString.Encode()
		_, err := utils.EndpointExec(&u)
		return err
	}

	data, err := ioutil.ReadFile(k.pidPath)
	if err != nil {
		logrus.WithField("path", k.pidPath).WithError(err).Error("Can't find keepalived")
		return err
	}

	pid, err := strconv.Atoi(strings.TrimSuffix(string(data), "\n"))
	if err != nil {
		logrus.WithField("path", k.pidPath).WithError(err).Error("Unknown pid found for keepalived")
		return err
	}

	proc, err := os.FindProcess(pid)
	if err != nil {
		logrus.WithField("pid", pid).WithError(err).Error("Failed to find process")
		return err
	}

	err = proc.Signal(syscall.Signal(signal))
	if err != nil {
		logrus.WithField("pid", pid).WithError(err).Error("Failed to send signal")
		return err
	}

	// Wait 10ms for Keepalived to create its files
	time.Sleep(10 * time.Millisecond)
	return nil
}
