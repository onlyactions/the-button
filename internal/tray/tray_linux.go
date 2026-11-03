//go:build linux && !android

// SPDX-License-Identifier: Unlicense OR MIT

package tray

import (
	"log"
	"time"

	"github.com/godbus/dbus/v5"

	"github.com/onlyactions/the-button/internal/gfx"
	"github.com/onlyactions/the-button/internal/state"
)

// StatusNotifierItem tray (the freedesktop spec used by KDE Plasma,
// GNOME's AppIndicator extension, most modern docks and stand-alone
// tray daemons). Left-click and middle-click both toggle the button.

const (
	sniIface     = "org.kde.StatusNotifierItem"
	sniPath      = dbus.ObjectPath("/StatusNotifierItem")
	propsIface   = "org.freedesktop.DBus.Properties"
	watcherName  = "org.kde.StatusNotifierWatcher"
	watcherIface = "org.kde.StatusNotifierWatcher"
	watcherPath  = dbus.ObjectPath("/StatusNotifierWatcher")

	iconSize = 32
)

// pixmap is the IconPixmap element: width, height, ARGB32 bytes.
type pixmap struct {
	W, H int32
	Data []byte
}

// tooltip matches the ToolTip property signature (sa(iiay)ss).
type tooltip struct {
	IconName string
	Pixmaps  []pixmap
	Title    string
	Desc     string
}

type item struct {
	st *state.State
}

// org.kde.StatusNotifierItem methods called by the tray host.

func (i *item) Activate(x, y int32) *dbus.Error {
	i.st.Toggle()
	return nil
}

func (i *item) SecondaryActivate(x, y int32) *dbus.Error {
	i.st.Toggle()
	return nil
}

func (i *item) Scroll(delta int32, dir string) *dbus.Error { return nil }
func (i *item) Refresh() *dbus.Error                       { return nil }

// org.freedesktop.DBus.Properties for the SNI interface. Values are
// computed live so they always reflect the current state.

func (i *item) Get(iface, prop string) (dbus.Variant, *dbus.Error) {
	if iface != sniIface {
		return dbus.Variant{}, &dbus.Error{
			Name: "org.freedesktop.DBus.Error.InvalidArgs",
			Body: []interface{}{"unknown interface " + iface},
		}
	}
	m := i.props()
	v, ok := m[prop]
	if !ok {
		return dbus.Variant{}, &dbus.Error{
			Name: "org.freedesktop.DBus.Error.InvalidArgs",
			Body: []interface{}{"unknown property " + prop},
		}
	}
	return v, nil
}

func (i *item) GetAll(iface string) (map[string]dbus.Variant, *dbus.Error) {
	if iface != sniIface {
		return nil, &dbus.Error{
			Name: "org.freedesktop.DBus.Error.InvalidArgs",
			Body: []interface{}{"unknown interface " + iface},
		}
	}
	return i.props(), nil
}

func (i *item) Set(iface, prop string, v dbus.Variant) *dbus.Error {
	return &dbus.Error{
		Name: "org.freedesktop.DBus.Error.PropertyReadOnly",
		Body: []interface{}{prop},
	}
}

func (i *item) props() map[string]dbus.Variant {
	on := i.st.IsOn()
	desc := "off"
	if on {
		desc = "on"
	}
	return map[string]dbus.Variant{
		"Category":   dbus.MakeVariant("ApplicationStatus"),
		"Id":         dbus.MakeVariant("the-button"),
		"Title":      dbus.MakeVariant("the button"),
		"Status":     dbus.MakeVariant("Active"),
		"IconName":   dbus.MakeVariant(""),
		"IconPixmap": dbus.MakeVariant([]pixmap{i.iconPixmap(on)}),
		"ToolTip":    dbus.MakeVariant(tooltip{Title: "the button", Desc: desc}),
		"ItemIsMenu": dbus.MakeVariant(false),
		"WindowId":   dbus.MakeVariant(int32(0)),
	}
}

func (i *item) iconPixmap(on bool) pixmap {
	img := gfx.Icon(iconSize, on, 0.45)
	return pixmap{W: iconSize, H: iconSize, Data: gfx.ARGB(img)}
}

// run installs the tray and serves it until the process ends. A
// missing session bus disables the tray silently; a missing watcher
// (GNOME without an extension, bare window managers) is retried
// until one shows up.
func run(st *state.State, _ string) {
	go func() {
		conn, err := dbus.ConnectSessionBus()
		if err != nil {
			log.Printf("tray: no session bus; tray disabled: %v", err)
			return
		}
		defer conn.Close()

		it := &item{st: st}
		if err := conn.Export(it, sniPath, sniIface); err != nil {
			log.Printf("tray: %v", err)
			return
		}
		if err := conn.Export(it, sniPath, propsIface); err != nil {
			log.Printf("tray: %v", err)
			return
		}

		// Re-color the tray icon whenever the state changes.
		st.OnChange(func(on bool) {
			// Nudge hosts that follow the New* signals...
			conn.Emit(sniPath, sniIface+".NewIcon")
			conn.Emit(sniPath, sniIface+".NewTitle")
			// ...and hosts that follow PropertiesChanged.
			conn.Emit(sniPath, propsIface+".PropertiesChanged", sniIface,
				map[string]dbus.Variant{
					"IconPixmap": dbus.MakeVariant([]pixmap{it.iconPixmap(on)}),
					"ToolTip":    dbus.MakeVariant(tooltip{Title: "the button", Desc: onDesc(on)}),
				}, []string(nil))
		})

		// Register with the watcher; retry while there isn't one.
		// Names()[0] is the bus-assigned unique name.
		service := ""
		if names := conn.Names(); len(names) > 0 {
			service = names[0]
		}
		for {
			obj := conn.Object(watcherName, watcherPath)
			if call := obj.Call(watcherIface+".RegisterStatusNotifierItem", 0, service); call.Err == nil {
				break
			}
			time.Sleep(5 * time.Second)
		}

		// Serve until the bus goes away.
		select {}
	}()
}

func onDesc(on bool) string {
	if on {
		return "on"
	}
	return "off"
}
