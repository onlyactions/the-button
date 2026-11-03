// SPDX-License-Identifier: Unlicense OR MIT

package com.onlyactions.thebutton;

import android.app.PendingIntent;
import android.appwidget.AppWidgetManager;
import android.appwidget.AppWidgetProvider;
import android.content.ComponentName;
import android.content.Context;
import android.content.Intent;
import android.widget.RemoteViews;

import com.onlyactions.thebutton.bridge.Bridge;

/**
 * Home-screen widget: one big button that toggles the same shared
 * state as the app and the quick-settings tile.
 */
public class ButtonWidget extends AppWidgetProvider {
    public static final String ACTION_TOGGLE = "com.onlyactions.thebutton.ACTION_TOGGLE";

    @Override
    public void onEnabled(Context context) {
        super.onEnabled(context);
        Sync.init(context.getApplicationContext());
    }

    @Override
    public void onUpdate(Context context, AppWidgetManager manager, int[] appWidgetIds) {
        Sync.init(context.getApplicationContext());
        for (int id : appWidgetIds) {
            push(context, manager, id);
        }
    }

    @Override
    public void onReceive(Context context, Intent intent) {
        if (ACTION_TOGGLE.equals(intent.getAction())) {
            Sync.init(context.getApplicationContext());
            Bridge.toggle();
            pushAll(context);
        } else {
            super.onReceive(context, intent);
        }
    }

    /** Re-renders every placed instance; called after any toggle. */
    static void pushAll(Context context) {
        AppWidgetManager manager = AppWidgetManager.getInstance(context);
        ComponentName provider = new ComponentName(context, ButtonWidget.class);
        for (int id : manager.getAppWidgetIds(provider)) {
            push(context, manager, id);
        }
    }

    private static void push(Context context, AppWidgetManager manager, int id) {
        boolean on = Bridge.isOn();
        RemoteViews views = new RemoteViews(context.getPackageName(), R.layout.widget_button);
        views.setImageViewResource(R.id.widget_icon, on ? R.drawable.widget_on : R.drawable.widget_off);
        views.setContentDescription(R.id.widget_icon,
                context.getString(R.string.widget_desc) + (on ? ": on" : ": off"));

        Intent toggle = new Intent(context, ButtonWidget.class);
        toggle.setAction(ACTION_TOGGLE);
        PendingIntent pi = PendingIntent.getBroadcast(context, 0, toggle,
                PendingIntent.FLAG_UPDATE_CURRENT | PendingIntent.FLAG_IMMUTABLE);
        views.setOnClickPendingIntent(R.id.widget_root, pi);

        manager.updateAppWidget(id, views);
    }
}
