// SPDX-FileCopyrightText: 2021 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package config

import (
	"github.com/Kristian-ZH/gardener-extension-logging/pkg/apis/config"
)

// Config contains configuration for the shoot logging service.
type Config struct {
	config.Configuration
}
