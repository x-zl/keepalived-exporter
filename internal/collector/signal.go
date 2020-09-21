package collector

import (
	"bytes"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/hashicorp/go-version"
	"github.com/sirupsen/logrus"
)

var sigNumSupportedVersion = version.Must(version.NewVersion("1.3.8"))

func isSigNumSupport(containerName string) bool {
	keepalivedVersion, err := getKeepalivedVersion(containerName)
	if err != nil {
		// keep backward compatibility and assuming it's the latest one on version detection failure
		return true
	}
	return keepalivedVersion.GreaterThanOrEqual(sigNumSupportedVersion)
}

func sigNum(sig, containerName string) int {
	if !isSigNumSupport(containerName) {
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

	if containerName != "" {
		outputCmd, err = dockerExecCmd(append([]string{"keepalived"}, sigNumCmd...), containerName)
		if err != nil {
			logrus.WithFields(logrus.Fields{"signal": sig, "container": containerName}).WithError(err).Fatal("Error getting signum")
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
		log.Panic(err)
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
		return dockerKillContainer(k.containerName, strconv.Itoa((signal)))
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
