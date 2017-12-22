package ecs

import (
	"fmt"
	"io"

	"github.com/BurntSushi/toml"
	"github.com/virtual-kubelet/virtual-kubelet/providers"
)

type providerConfig struct {
	Region          string
	AccessKey       string
	SecretKey       string
	OperatingSystem string
	CPU             string
	Memory          string
	Pods            string
	LaunchType      string
	Subnet          string
	SecurityGroup   string
	ClusterName     string
}

func (p *ECSProvider) loadConfig(r io.Reader) error {
	var config providerConfig
	if _, err := toml.DecodeReader(r, &config); err != nil {
		return err
	}
	p.region = config.Region

	p.securityGroup = config.SecurityGroup
	p.subnet = config.Subnet
	p.clusterName = config.ClusterName

	// Default to Linux if the operating system was not defined in the config.
	if config.OperatingSystem == "" {
		config.OperatingSystem = providers.OperatingSystemLinux
	}

	// Validate operating system from config.
	if config.OperatingSystem != providers.OperatingSystemLinux {
		return fmt.Errorf("%q is not a valid operating system, only %s is valid", config.OperatingSystem, providers.OperatingSystemLinux)
	}

	p.operatingSystem = config.OperatingSystem
	return nil
}
