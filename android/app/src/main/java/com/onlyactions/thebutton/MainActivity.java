// SPDX-License-Identifier: Unlicense OR MIT

package com.onlyactions.thebutton;

import android.app.Activity;
import android.os.Bundle;

import com.onlyactions.thebutton.bridge.Bridge;

import org.gioui.Gio;
import org.gioui.GioView;

/**
 * The button itself: a full-screen Gio view.
 *
 * The Gio window (created once on the Go side by Bridge.startView)
 * attaches to this activity's GioView and survives activity
 * re-creation.
 */
public class MainActivity extends Activity {
    private GioView view;

    @Override
    protected void onCreate(Bundle savedInstanceState) {
        super.onCreate(savedInstanceState);
        Sync.init(getApplicationContext());
        Gio.init(getApplicationContext());
        Bridge.startView();
        view = new GioView(this);
        setContentView(view);
    }

    @Override
    protected void onStart() {
        super.onStart();
        view.start();
    }

    @Override
    protected void onStop() {
        view.stop();
        super.onStop();
    }

    @Override
    protected void onPause() {
        super.onPause();
        view.pause();
    }

    @Override
    protected void onResume() {
        view.resume();
        super.onResume();
    }

    @Override
    protected void onDestroy() {
        view.destroy();
        super.onDestroy();
    }

    @Override
    public void onLowMemory() {
        super.onLowMemory();
        GioView.onLowMemory();
    }
}
