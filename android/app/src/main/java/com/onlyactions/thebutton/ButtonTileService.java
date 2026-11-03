// SPDX-License-Identifier: Unlicense OR MIT

package com.onlyactions.thebutton;

import android.service.quicksettings.Tile;
import android.service.quicksettings.TileService;

import com.onlyactions.thebutton.bridge.Bridge;

/**
 * Quick-settings tile (pull down the notification shade): a red/green
 * button that toggles the same shared state as the app and the widget.
 */
public class ButtonTileService extends TileService {
    @Override
    public void onCreate() {
        super.onCreate();
        Sync.init(getApplicationContext());
    }

    @Override
    public void onStartListening() {
        update();
    }

    @Override
    public void onClick() {
        Bridge.toggle();
        update();
    }

    private void update() {
        Tile tile = getQsTile();
        if (tile == null) {
            return;
        }
        boolean on = Bridge.isOn();
        tile.setState(on ? Tile.STATE_ACTIVE : Tile.STATE_INACTIVE);
        tile.setIcon(android.graphics.drawable.Icon.createWithResource(
                this, on ? R.drawable.tile_on : R.drawable.tile_off));
        tile.setLabel(getString(R.string.tile_label));
        tile.updateTile();
    }
}
