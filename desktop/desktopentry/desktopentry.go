// -*- Mode: Go; indent-tabs-mode: t -*-

/*
 * Copyright (C) 2023 Canonical Ltd
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU General Public License version 3 as
 * published by the Free Software Foundation.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU General Public License for more details.
 *
 * You should have received a copy of the GNU General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 *
 */

package desktopentry

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/snapcore/snapd/strutil"
)

type DesktopEntry struct {
	Filename string
	Name     string
	Icon     string
	Exec     string

	Hidden                bool
	OnlyShowIn            []string
	NotShownIn            []string
	GnomeAutostartEnabled bool

	Actions map[string]*Action
}

type Action struct {
	Name string
	Icon string
	Exec string
}

type groupState int

const (
	unknownGroup groupState = iota
	desktopEntryGroup
	desktopActionGroup
)

func splitStringList(value string) []string {
	return strings.FieldsFunc(value, func(r rune) bool { return r == ';' })
}

func Load(filename string) (*DesktopEntry, error) {
	f, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return parse(filename, f)
}

func parse(filename string, r io.Reader) (*DesktopEntry, error) {
	de := &DesktopEntry{
		Filename:              filename,
		GnomeAutostartEnabled: true,
	}
	var (
		currentGroup          = unknownGroup
		seenDesktopEntryGroup = false
		actions               []string
		currentAction         *Action
	)

	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		// Ignore empty lines and comments
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		// Start of a new group
		if strings.HasPrefix(line, "[") {
			if line == "[Desktop Entry]" {
				if seenDesktopEntryGroup {
					return nil, fmt.Errorf("desktop file %q has multiple [Desktop Entry] groups", filename)
				}
				seenDesktopEntryGroup = true
				currentGroup = desktopEntryGroup
			} else if strings.HasPrefix(line, "[Desktop Action ") && strings.HasSuffix(line, "]") {
				action := line[len("[Desktop Action ") : len(line)-1]
				if !strutil.ListContains(actions, action) {
					return nil, fmt.Errorf("desktop file %q contains unknown action %q", filename, action)
				}
				if de.Actions[action] != nil {
					return nil, fmt.Errorf("desktop file %q has multiple %q groups", filename, line)
				}
				currentGroup = desktopActionGroup
				if de.Actions == nil {
					de.Actions = make(map[string]*Action, len(actions))
				}
				currentAction = &Action{}
				de.Actions[action] = currentAction
			} else {
				// Ignore other groups
				currentGroup = unknownGroup
			}
			continue
		}

		split := strings.SplitN(line, "=", 2)
		if len(split) != 2 {
			return nil, fmt.Errorf("desktop file %q badly formed", filename)
		}
		// Trim whitespace around the equals sign
		key := strings.TrimRight(split[0], "\t\n\v\f\r ")
		value := strings.TrimLeft(split[1], "\t\n\v\f\r ")
		switch currentGroup {
		case desktopEntryGroup:
			switch key {
			case "Name":
				de.Name = value
			case "Icon":
				de.Icon = value
			case "Exec":
				de.Exec = value
			case "Hidden":
				de.Hidden = value == "true"
			case "OnlyShowIn":
				de.OnlyShowIn = splitStringList(value)
			case "NotShownIn":
				de.NotShownIn = splitStringList(value)
			case "X-GNOME-Autostart-enabled":
				de.GnomeAutostartEnabled = value == "true"
			case "Actions":
				actions = splitStringList(value)
			}
		case desktopActionGroup:
			switch key {
			case "Name":
				currentAction.Name = value
			case "Icon":
				currentAction.Icon = value
			case "Exec":
				currentAction.Exec = value
			}
		}
	}
	return de, nil
}

func isOneOfIn(of []string, other []string) bool {
	for _, one := range of {
		if strutil.ListContains(other, one) {
			return true
		}
	}
	return false
}

// ShouldAutostart returns true if this desktop file should autostart
// on the given desktop.
//
// currentDesktop is the value of $XDG_CURRENT_DESKTOP split on colon
// characters.
func (de *DesktopEntry) ShouldAutostart(currentDesktop []string) bool {
	// See https://standards.freedesktop.org/autostart-spec/autostart-spec-latest.html
	// for details on how Hidden, OnlyShowIn, NotShownIn are handled.

	if de.Hidden {
		return false
	}
	if !de.GnomeAutostartEnabled {
		// GNOME specific extension, see gnome-session:
		// https://github.com/GNOME/gnome-session/blob/c449df5269e02c59ae83021a3110ec1b338a2bba/gnome-session/gsm-autostart-app.c#L110..L145
		if strutil.ListContains(currentDesktop, "GNOME") {
			return false
		}
	}
	if de.OnlyShowIn != nil {
		if !isOneOfIn(currentDesktop, de.OnlyShowIn) {
			return false
		}
	}
	if de.NotShownIn != nil {
		if isOneOfIn(currentDesktop, de.NotShownIn) {
			return false
		}
	}
	return true
}

func (de *DesktopEntry) ExpandExec(uris []string) ([]string, error) {
	if de.Exec == "" {
		return nil, fmt.Errorf("desktop file %q has no Exec line", de.Filename)
	}
	return expandExec(de, de.Exec, uris)
}

func (de *DesktopEntry) ExpandActionExec(action string, uris []string) ([]string, error) {
	if de.Actions[action] == nil {
		return nil, fmt.Errorf("desktop file %q does not have action %q", de.Filename, action)
	}
	if de.Actions[action].Exec == "" {
		return nil, fmt.Errorf("desktop file %q action %q has no Exec line", de.Filename, action)
	}
	return expandExec(de, de.Actions[action].Exec, uris)
}
