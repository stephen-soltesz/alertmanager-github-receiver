# alertmanager-github-receiver

Notes:

 * AM sends "firing" and "resolved" messages.
 * No notifications for silences.
 * Alerts are grouped by the 'groupLabels'.

   - there can be multiple specific alerts within a single set of groupLabels.

 * Silences apply to individual alerts.
 * AM will automatically resolve an alert if there's not another notification
   from prometheus (or another alert source).

 * Issues created from alerts that we want to silence (b/c we won't resolve
   them), must be manually closed on github.
