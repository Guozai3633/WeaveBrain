package com.weavebrain.weave_flutter

import android.appwidget.AppWidgetManager
import android.content.Context
import android.content.SharedPreferences
import android.net.Uri
import android.widget.RemoteViews
import es.antonborri.home_widget.HomeWidgetLaunchIntent
import es.antonborri.home_widget.HomeWidgetProvider

/**
 * 桌面「速记」小组件：整块点按即以 `weavebrain://quick_capture` 启动主界面，
 * Flutter 侧 [DeviceHomeWidgetService] 会把任意小组件启动归一为「极简录音」。
 */
class EchoCaptureWidgetProvider : HomeWidgetProvider() {

    override fun onUpdate(
        context: Context,
        appWidgetManager: AppWidgetManager,
        appWidgetIds: IntArray,
        widgetData: SharedPreferences,
    ) {
        appWidgetIds.forEach { widgetId ->
            val views =
                RemoteViews(context.packageName, R.layout.echo_capture_widget).apply {
                    val pendingIntent =
                        HomeWidgetLaunchIntent.getActivity(
                            context,
                            MainActivity::class.java,
                            Uri.parse("weavebrain://quick_capture"),
                        )
                    setOnClickPendingIntent(R.id.echo_capture_widget_root, pendingIntent)
                }
            appWidgetManager.updateAppWidget(widgetId, views)
        }
    }
}
