// Copyright 2015, 2018 CoreOS, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Came from https://github.com/coreos/go-systemd/blob/main/dbus/dbus.go

package systemd

import (
	"context"
	"fmt"

	"github.com/godbus/dbus/v5"
)

// Wrapper to add helper methods that render native types in a simpler way.
type Variant struct {
	v *dbus.Variant
}

func (va Variant) String() string {
	s, err := va.AsString()
	if err != nil {
		return fmt.Sprintf("%+v", va.v.Value())
	}
	return s
}

func (va Variant) AsString() (string, error) {
	var s string
	if err := va.v.Store(&s); err != nil {
		return "", err
	}
	return s, nil
}

func (va Variant) AsInt() (int, error) {
	var i int
	err := va.v.Store(&i)
	return i, err
}

func Connect(ctx context.Context) (*Conn, error) {
	conn, err := dbus.SystemBusPrivate(dbus.WithContext(ctx))
	if err != nil {
		return nil, err
	}
	if err := conn.Auth(nil); err != nil {
		return nil, err
	}
	if err := conn.Hello(); err != nil {
		return nil, err
	}
	return &Conn{c: conn}, nil
}

type Conn struct {
	c *dbus.Conn
}

func (conn *Conn) Close() error { return conn.c.Close() }

func (conn *Conn) GetManagerProperty(prop string) (Variant, error) {
	variant := Variant{}
	obj := conn.c.Object("org.freedesktop.systemd1", dbus.ObjectPath("/org/freedesktop/systemd1"))
	err := obj.Call("org.freedesktop.DBus.Properties.Get", 0, "org.freedesktop.systemd1.Manager", prop).Store(&variant.v)
	return variant, err
}

type UnitStatus struct {
	Name        string          // The primary unit name as string
	Description string          // The human readable description string
	LoadState   string          // The load state (i.e. whether the unit file has been loaded successfully)
	ActiveState string          // The active state (i.e. whether the unit is currently started or not)
	SubState    string          // The sub state (a more fine-grained version of the active state that is specific to the unit type, which the active state is not)
	Followed    string          // A unit that is being followed in its state by this unit, if there is any, otherwise the empty string.
	Path        dbus.ObjectPath // The unit object path
	JobId       uint32          // If there is a job queued for the job unit the numeric job id, 0 otherwise
	JobType     string          // The job type as string
	JobPath     dbus.ObjectPath // The job object path
}

func (conn *Conn) ListUnitsFiltered(filter []string) ([]UnitStatus, error) {
	var result [][]interface{}
	obj := conn.c.Object("org.freedesktop.systemd1", dbus.ObjectPath("/org/freedesktop/systemd1"))
	call := obj.Call("org.freedesktop.systemd1.Manager.ListUnitsFiltered", 0, filter)
	if call.Err != nil {
		return nil, call.Err
	}
	err := call.Store(&result)

	resultInterface := make([]interface{}, len(result))
	for i := range result {
		resultInterface[i] = result[i]
	}

	units := make([]UnitStatus, len(result))
	statusInterface := make([]interface{}, len(units))
	for i := range units {
		statusInterface[i] = &units[i]
	}

	err = dbus.Store(resultInterface, statusInterface...)
	if err != nil {
		return nil, err
	}

	return units, nil
}
