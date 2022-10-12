// SPDX-FileCopyrightText: 2021 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package cmd

import (
	"errors"
	"os"

	apisconfig "github.com/Kristian-ZH/gardener-extension-logging/pkg/apis/config"
	"github.com/Kristian-ZH/gardener-extension-logging/pkg/apis/config/v1alpha1"
	controllerconfig "github.com/Kristian-ZH/gardener-extension-logging/pkg/controller/config"
	healthcheckcontroller "github.com/Kristian-ZH/gardener-extension-logging/pkg/controller/healthcheck"
	"github.com/Kristian-ZH/gardener-extension-logging/pkg/controller/lifecycle"
	webhook "github.com/Kristian-ZH/gardener-extension-logging/pkg/webhook/osc"
	healthcheckconfig "github.com/gardener/gardener/extensions/pkg/apis/config"
	"github.com/gardener/gardener/extensions/pkg/controller/cmd"
	extensionshealthcheckcontroller "github.com/gardener/gardener/extensions/pkg/controller/healthcheck"
	webhookcmd "github.com/gardener/gardener/extensions/pkg/webhook/cmd"
	"github.com/spf13/pflag"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"

	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
)

// WebhookSwitchOptions are the webhookcmd.SwitchOptions for the logging webhooks.
func WebhookSwitchOptions() *webhookcmd.SwitchOptions {
	return webhookcmd.NewSwitchOptions(
		webhookcmd.Switch("logging", webhook.New),
	)
}

var (
	scheme  *runtime.Scheme
	decoder runtime.Decoder
)

func init() {
	scheme = runtime.NewScheme()
	utilruntime.Must(apisconfig.AddToScheme(scheme))
	utilruntime.Must(v1alpha1.AddToScheme(scheme))

	decoder = serializer.NewCodecFactory(scheme).UniversalDecoder()
}

// LoggingServiceOptions holds options related to the Logging service.
type LoggingServiceOptions struct {
	ConfigLocation string
	config         *LoggingServiceConfig
}

// AddFlags implements Flagger.AddFlags.
func (o *LoggingServiceOptions) AddFlags(fs *pflag.FlagSet) {
	fs.StringVar(&o.ConfigLocation, "config", "", "Path to logging service configuration")
}

// Complete implements Completer.Complete.
func (o *LoggingServiceOptions) Complete() error {
	if o.ConfigLocation == "" {
		return errors.New("config location is not set")
	}

	data, err := os.ReadFile(o.ConfigLocation)
	if err != nil {
		return err
	}

	config := apisconfig.Configuration{}
	_, _, err = decoder.Decode(data, nil, &config)
	if err != nil {
		return err
	}

	o.config = &LoggingServiceConfig{
		config: config,
	}

	return nil
}

// Completed returns the decoded LoggingServiceConfiguration instance. Only call this if `Complete` was successful.
func (o *LoggingServiceOptions) Completed() *LoggingServiceConfig {
	return o.config
}

// LoggingServiceConfig contains configuration information about the Logging service.
type LoggingServiceConfig struct {
	config apisconfig.Configuration
}

// Apply applies the LoggingServiceOptions to the passed ControllerOptions instance.
func (c *LoggingServiceConfig) Apply(config *controllerconfig.Config) {
	config.Configuration = c.config
}

// ApplyHealthCheckConfig applies the HealthCheckConfig to the config.
func (c *LoggingServiceConfig) ApplyHealthCheckConfig(config *healthcheckconfig.HealthCheckConfig) {
	if c.config.HealthCheckConfig != nil {
		*config = *c.config.HealthCheckConfig
	}
}

// ControllerSwitches are the cmd.ControllerSwitches for the extension controllers.
func ControllerSwitches() *cmd.SwitchOptions {
	return cmd.NewSwitchOptions(
		cmd.Switch(lifecycle.Name, lifecycle.AddToManager),
		cmd.Switch(extensionshealthcheckcontroller.ControllerName, healthcheckcontroller.AddToManager),
	)
}
