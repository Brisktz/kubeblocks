/*
Copyright (C) 2022-2023 ApeCloud Co., Ltd

This file is part of KubeBlocks project

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.

This program is distributed in the hope that it will be useful
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program.  If not, see <http://www.gnu.org/licenses/>.
*/

package hypervisor

import (
	"github.com/dapr/kit/logger"
	"github.com/spf13/pflag"
)

type Hypervisor struct {
	Logger  logger.Logger
	Daemon  *Daemon
	Watcher *Watcher
}

func NewHypervisor(logger logger.Logger) (*Hypervisor, error) {
	args := pflag.Args()
	daemon, err := NewDeamon(args, logger)
	if err != nil {
		return nil, err
	}

	watcher := NewWatcher(logger)
	go watcher.Start()
	hypervisor := &Hypervisor{
		Logger:  logger,
		Daemon:  daemon,
		Watcher: watcher,
	}

	return hypervisor, nil
}

func (hypervisor *Hypervisor) Start() {
	if hypervisor.Daemon == nil {
		hypervisor.Logger.Info("No DB Service")
		return
	}

	err := hypervisor.Daemon.Start()
	if err != nil {
		hypervisor.Logger.Warnf("Start DB Service failed: %s", err)
		return
	}

	hypervisor.Watcher.Watch(hypervisor.Daemon)
}
