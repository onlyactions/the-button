// SPDX-License-Identifier: Unlicense OR MIT

package com.onlyactions.thebutton;

import android.content.ComponentName;
import android.content.Context;
import android.os.Build;
import android.os.Handler;
import android.os.Looper;
import android.service.quicksettings.TileService;

import com.onlyactions.thebutton.bridge.Bridge;
import com.onlyactions.thebutton.bridge.Listener;

/**
 * One-shot initialization plus the Go → Java change listener: when the
 * state flips anywhere (app window, tile, widget, desktop tray...), the
 * widget re-renders and the tile refreshes.
 */
public final class Sync implements Listener {
    private static Sync instance;

    private final Context ctx;
    private final Handler main = new Handler(Looper.getMainLooper());

    private Sync(Context appContext) {
        this.ctx = appContext;
    }

    /** Idempotent; safe to call from every entry point. */
    public static synchronized void init(Context appContext) {
        if (instance == null) {
            Sync s = new Sync(appContext);
            Bridge.setup(appContext.getFilesDir().getAbsolutePath());
            instance = s;
            Bridge.setListener(s);
        }
    }

    // Called from the Go side on whatever thread toggled the state.
    @Override
    public void stateChanged(boolean on) {
        main.post(new Runnable() {
            @Override
            public void run() {
                ButtonWidget.pushAll(ctx);
                if (Build.VERSION.SDK_INT >= 28) {
                    TileService.requestListeningState(ctx,
                            new ComponentName(ctx, ButtonTileService.class));
                }
            }
        });
    }
}
