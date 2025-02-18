// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package system // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal/system"

import (
	"context"
	"errors"
	"fmt"
	"strconv"

	"github.com/shirou/gopsutil/v3/cpu"
	"go.opentelemetry.io/collector/featuregate"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/processor"
	conventions "go.opentelemetry.io/collector/semconv/v1.6.1"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/metadataproviders/system"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal/system/internal/metadata"
)

var (
	hostCPUModelAndFamilyAsStringID          = "processor.resourcedetection.hostCPUModelAndFamilyAsString"
	hostCPUModelAndFamilyAsStringFeatureGate = featuregate.GlobalRegistry().MustRegister(
		hostCPUModelAndFamilyAsStringID,
		featuregate.StageAlpha,
		featuregate.WithRegisterDescription("Change type of host.cpu.model.id and host.cpu.model.family to string."),
		featuregate.WithRegisterFromVersion("v0.89.0"),
		featuregate.WithRegisterReferenceURL("https://github.com/open-telemetry/semantic-conventions/issues/495"),
	)
)

const (
	// TypeStr is type of detector.
	TypeStr = "system"
)

var hostnameSourcesMap = map[string]func(*Detector) (string, error){
	"os":     getHostname,
	"dns":    getFQDN,
	"cname":  lookupCNAME,
	"lookup": reverseLookupHost,
}

var _ internal.Detector = (*Detector)(nil)

// Detector is a system metadata detector
type Detector struct {
	provider system.Provider
	logger   *zap.Logger
	cfg      Config
	rb       *metadata.ResourceBuilder
}

// NewDetector creates a new system metadata detector
func NewDetector(p processor.CreateSettings, dcfg internal.DetectorConfig) (internal.Detector, error) {
	cfg := dcfg.(Config)
	if len(cfg.HostnameSources) == 0 {
		cfg.HostnameSources = []string{"dns", "os"}
	}

	return &Detector{
		provider: system.NewProvider(),
		logger:   p.Logger,
		cfg:      cfg,
		rb:       metadata.NewResourceBuilder(cfg.ResourceAttributes),
	}, nil
}

// Detect detects system metadata and returns a resource with the available ones
func (d *Detector) Detect(ctx context.Context) (resource pcommon.Resource, schemaURL string, err error) {
	var hostname string

	osType, err := d.provider.OSType()
	if err != nil {
		return pcommon.NewResource(), "", fmt.Errorf("failed getting OS type: %w", err)
	}

	hostArch, err := d.provider.HostArch()
	if err != nil {
		return pcommon.NewResource(), "", fmt.Errorf("failed getting host architecture: %w", err)
	}

	osDescription, err := d.provider.OSDescription(ctx)
	if err != nil {
		return pcommon.NewResource(), "", fmt.Errorf("failed getting OS description: %w", err)
	}

	cpuInfo, err := cpu.Info()
	if err != nil {
		return pcommon.NewResource(), "", fmt.Errorf("failed getting host cpuinfo: %w", err)
	}

	for _, source := range d.cfg.HostnameSources {
		getHostFromSource := hostnameSourcesMap[source]
		hostname, err = getHostFromSource(d)
		if err == nil {
			d.rb.SetHostName(hostname)
			d.rb.SetOsType(osType)
			if d.cfg.ResourceAttributes.HostID.Enabled {
				if hostID, hostIDErr := d.provider.HostID(ctx); hostIDErr == nil {
					d.rb.SetHostID(hostID)
				} else {
					d.logger.Warn("failed to get host ID", zap.Error(hostIDErr))
				}
			}
			d.rb.SetHostArch(hostArch)
			d.rb.SetOsDescription(osDescription)
			if len(cpuInfo) > 0 {
				err = setHostCPUInfo(d, cpuInfo[0])
				if err != nil {
					d.logger.Warn("failed to get host cpuinfo", zap.Error(err))
				}
			}
			return d.rb.Emit(), conventions.SchemaURL, nil
		}
		d.logger.Debug(err.Error())
	}

	return pcommon.NewResource(), "", errors.New("all hostname sources failed to get hostname")
}

// getHostname returns OS hostname
func getHostname(d *Detector) (string, error) {
	hostname, err := d.provider.Hostname()
	if err != nil {
		return "", fmt.Errorf("failed getting OS hostname: %w", err)
	}
	return hostname, nil
}

// getFQDN returns FQDN of the host
func getFQDN(d *Detector) (string, error) {
	hostname, err := d.provider.FQDN()
	if err != nil {
		return "", fmt.Errorf("getFQDN failed getting FQDN: %w", err)
	}
	return hostname, nil
}

func lookupCNAME(d *Detector) (string, error) {
	cname, err := d.provider.LookupCNAME()
	if err != nil {
		return "", fmt.Errorf("lookupCNAME failed to get CNAME: %w", err)
	}
	return cname, nil
}

func reverseLookupHost(d *Detector) (string, error) {
	hostname, err := d.provider.ReverseLookupHost()
	if err != nil {
		return "", fmt.Errorf("reverseLookupHost failed to lookup host: %w", err)
	}
	return hostname, nil
}

func setHostCPUInfo(d *Detector, cpuInfo cpu.InfoStat) error {
	d.logger.Debug("getting host's cpuinfo", zap.String("coreID", cpuInfo.CoreID))
	d.rb.SetHostCPUVendorID(cpuInfo.VendorID)
	if hostCPUModelAndFamilyAsStringFeatureGate.IsEnabled() {
		d.rb.SetHostCPUFamily(cpuInfo.Family)
	} else {
		// https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/29025
		d.logger.Warn("This attribute will change from int to string. Switch now using the feature gate.",
			zap.String("attribute", "host.cpu.family"),
			zap.String("feature gate", hostCPUModelAndFamilyAsStringID),
		)
		family, err := strconv.ParseInt(cpuInfo.Family, 10, 64)
		if err != nil {
			return fmt.Errorf("failed to convert cpuinfo family to integer: %w", err)
		}
		d.rb.SetHostCPUFamilyAsInt(family)
	}

	// For windows, this field is left blank. See https://github.com/shirou/gopsutil/blob/v3.23.9/cpu/cpu_windows.go#L113
	// Skip setting modelId if the field is blank.
	// ISSUE: https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/27675
	if cpuInfo.Model != "" {
		if hostCPUModelAndFamilyAsStringFeatureGate.IsEnabled() {
			d.rb.SetHostCPUModelID(cpuInfo.Model)
		} else {
			// https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/29025
			d.logger.Warn("This attribute will change from int to string. Switch now using the feature gate.",
				zap.String("attribute", "host.cpu.model.id"),
				zap.String("feature gate", hostCPUModelAndFamilyAsStringID),
			)
			model, err := strconv.ParseInt(cpuInfo.Model, 10, 64)
			if err != nil {
				return fmt.Errorf("failed to convert cpuinfo model to integer: %w", err)
			}
			d.rb.SetHostCPUModelIDAsInt(model)
		}
	}

	d.rb.SetHostCPUModelName(cpuInfo.ModelName)
	d.rb.SetHostCPUStepping(int64(cpuInfo.Stepping))
	d.rb.SetHostCPUCacheL2Size(int64(cpuInfo.CacheSize))
	return nil
}
