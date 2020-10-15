package main

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/mattermost/mattermost-server/v5/plugin"
	"github.com/ulumuri/go-nagios/nagios"
)

type commandHandlerFunc func(p *Plugin, channelID string, parameters []string) string

// TODO(DanielSz50): implement get-current-limits command

const (
	logErrorKey = "error"

	settingLogsLimitUnsuccessful = "Setting logs limit unsuccessful."
	logsLimitKey                 = "logs-limit"
	defaultLogsLimit             = 50

	settingLogsStartTimeUnsuccessful = "Setting logs start time unsuccessful."
	logsStartTimeKey                 = "logs-start-time"
	defaultLogsStartTime             = 86400 // get logs from one day

	gettingLogsUnsuccessful = "Getting logs unsuccessful"
	resultTypeTextSuccess   = "Success"

	settingReportFrequencyUnsuccessful = "Setting report frequency unsuccessful."
	reportFrequencyKey                 = "report-frequency"
	defaultReportFrequency             = 10

	settingReportChannelUnsuccessful = "Setting system monitoring report channel unsuccessful."
	reportChannelKey                 = "report-channel"

	settingChangesChannelUnsuccessful = "Setting configuration changes channel unsuccessful."
	changesChannelKey                 = "changes-channel"
)

func getLogsLimit(api plugin.API) (int, error) {
	b, err := api.KVGet(logsLimitKey)
	if err != nil {
		return 0, fmt.Errorf("api.KVGet: %w", err)
	}

	var limit int

	if err := json.Unmarshal(b, &limit); err != nil {
		return 0, fmt.Errorf("json.Unmarshal: %w", err)
	}

	return limit, nil
}

func (p *Plugin) setLogsLimit(parameters []string) string {
	if len(parameters) != 1 {
		return "You must supply exactly one parameter (integer value)."
	}

	i, err := strconv.Atoi(parameters[0])
	if err != nil {
		p.API.LogError("Atoi", logErrorKey, err)
		return settingLogsLimitUnsuccessful
	}

	if i <= 0 {
		return "Invalid argument - logs limit must be a positive integer."
	}

	b, err := json.Marshal(i)
	if err != nil {
		p.API.LogError("Marshal", logErrorKey, err)
		return settingLogsLimitUnsuccessful
	}

	if err := p.API.KVSet(logsLimitKey, b); err != nil {
		p.API.LogError("KVSet", logErrorKey, err)
		return settingLogsLimitUnsuccessful
	}

	return "Limit set successfully."
}

func setLogsLimit(p *Plugin, channelID string, parameters []string) string {
	return p.setLogsLimit(parameters)
}

func getLogsStartTime(api plugin.API) (time.Duration, error) {
	b, err := api.KVGet(logsStartTimeKey)
	if err != nil {
		return 0, fmt.Errorf("api.KVGet: %w", err)
	}

	var seconds int64

	if err := json.Unmarshal(b, &seconds); err != nil {
		return 0, fmt.Errorf("json.Unmarshal: %w", err)
	}

	return time.Duration(seconds) * time.Second, nil
}

func (p *Plugin) setLogsStartTime(parameters []string) string {
	if len(parameters) != 1 {
		return "You must supply exactly one parameter (number of seconds)."
	}

	i, err := strconv.ParseInt(parameters[0], 10, 64)
	if err != nil {
		p.API.LogError("ParseInt", logErrorKey, err)
		return settingLogsStartTimeUnsuccessful
	}

	if i <= 0 {
		return "Invalid argument - start time must be a positive integer."
	}

	b, err := json.Marshal(i)
	if err != nil {
		p.API.LogError("Marshal", logErrorKey, err)
		return settingLogsStartTimeUnsuccessful
	}

	if err := p.API.KVSet(logsStartTimeKey, b); err != nil {
		p.API.LogError("KVSet", logErrorKey, err)
		return settingLogsStartTimeUnsuccessful
	}

	return "Start time set successfully."
}

func setLogsStartTime(p *Plugin, channelID string, parameters []string) string {
	return p.setLogsStartTime(parameters)
}

// formatNagiosTimestamp formats the timestamp from Nagios Core JSON CGIs
// output. These CGIs return the number of milliseconds since the Unix Epoch
// (hence division by 1000). This is contrary to what these CGIs consume, which
// is the _number of seconds_ since the Unix Epoch.
func formatNagiosTimestamp(t int64) string {
	return time.Unix(t/1e3, 0).String()
}

func formatHostName(name, alt string) string {
	if len(name) == 0 {
		return alt
	}
	return name
}

func gettingLogsUnsuccessfulMessage(message string) string {
	return fmt.Sprintf("%s: %s", gettingLogsUnsuccessful, message)
}

func unknownParameterMessage(parameter string) string {
	return fmt.Sprintf("Unknown parameter (%s).", parameter)
}

func formatAlertListEntry(e nagios.AlertListEntry) string {
	return fmt.Sprintf("%s [%s] %s: %s | %s | %s | %s | %s",
		emoji(e.State),
		formatNagiosTimestamp(e.Timestamp),
		e.ObjectType,
		formatHostName(e.HostName, e.Name),
		e.Description,
		e.StateType,
		e.State,
		e.PluginOutput)
}

func formatAlerts(alerts nagios.AlertList) string {
	if alerts.Result.TypeText != resultTypeTextSuccess {
		return gettingLogsUnsuccessfulMessage(alerts.Result.TypeText)
	}

	if len(alerts.Data.AlertList) == 0 {
		return "No alerts."
	}

	var b strings.Builder

	for i, v := range alerts.Data.AlertList {
		if i > 0 {
			b.WriteRune('\n')
		}
		b.WriteString(formatAlertListEntry(v))
	}

	return b.String()
}

func formatNotificationListEntry(e nagios.NotificationListEntry) string {
	return fmt.Sprintf("[%s] %s: %s | %s | %s | %s | %s | %s",
		formatNagiosTimestamp(e.Timestamp),
		e.ObjectType,
		formatHostName(e.HostName, e.Name),
		e.Description,
		e.Contact,
		e.NotificationType,
		e.Method,
		e.Message)
}

func formatNotifications(notifications nagios.NotificationList) string {
	if notifications.Result.TypeText != resultTypeTextSuccess {
		return gettingLogsUnsuccessfulMessage(notifications.Result.TypeText)
	}

	if len(notifications.Data.NotificationList) == 0 {
		return "No notifications."
	}

	var b strings.Builder

	for i, v := range notifications.Data.NotificationList {
		if i > 0 {
			b.WriteRune('\n')
		}
		b.WriteString(formatNotificationListEntry(v))
	}

	return b.String()
}

// Cheat sheet:
//
// [command] [action]      [parameters...]
// get-log   alerts        <host>    <URL>
// get-log   alerts        <service> <SVC>
// get-log   notifications <host>    <URL>
// get-log   notifications <service> <SVC>

func getLogsSpecific(parameters []string) (hostName, serviceDescription, message string, ok bool) {
	if len(parameters) == 0 {
		return "", "", "", true
	}

	switch parameters[0] {
	case "host":
		if len(parameters) < 2 {
			return "", "", "You must supply host name.", false
		}
		return parameters[1], "", "", true
	case "service":
		if len(parameters) < 2 {
			return "", "", "You must supply service description.", false
		}
		return "", parameters[1], "", true
	default:
		return "", "", unknownParameterMessage(parameters[0]), false
	}
}

func (p *Plugin) getLogs(parameters []string) string {
	if len(parameters) == 0 {
		return "You must supply at least one parameter (alerts|notifications)."
	}

	c, err := getLogsLimit(p.API)
	if err != nil {
		p.API.LogError("getLogsLimit", logErrorKey, err)
		return gettingLogsUnsuccessful
	}

	hostName, serviceDescription, message, ok := getLogsSpecific(parameters[1:])
	if !ok {
		return message
	}

	d, err := getLogsStartTime(p.API)
	if err != nil {
		p.API.LogError("getLogsStartTime", logErrorKey, err)
		return gettingLogsUnsuccessful
	}

	now := time.Now()
	then := now.Add(-d)

	switch parameters[0] {
	case "alerts":
		q := nagios.AlertListRequest{
			GeneralAlertRequest: nagios.GeneralAlertRequest{
				FormatOptions: nagios.FormatOptions{
					Enumerate: true,
				},
				Count:              c,
				HostName:           hostName,
				ServiceDescription: serviceDescription,
				StartTime:          then.Unix(),
				EndTime:            now.Unix(),
			},
		}
		var alerts nagios.AlertList
		if err := p.client.Query(q, &alerts); err != nil {
			p.API.LogError("Query", logErrorKey, err)
			return gettingLogsUnsuccessful
		}
		return formatAlerts(alerts)
	case "notifications":
		q := nagios.NotificationListRequest{
			GeneralNotificationRequest: nagios.GeneralNotificationRequest{
				FormatOptions: nagios.FormatOptions{
					Enumerate: true,
				},
				Count:              c,
				HostName:           hostName,
				ServiceDescription: serviceDescription,
				StartTime:          then.Unix(),
				EndTime:            now.Unix(),
			},
		}
		var notifications nagios.NotificationList
		if err := p.client.Query(q, &notifications); err != nil {
			p.API.LogError("Query", logErrorKey, err)
			return gettingLogsUnsuccessful
		}
		return formatNotifications(notifications)
	default:
		return unknownParameterMessage(parameters[0])
	}
}

func getLogs(p *Plugin, channelID string, parameters []string) string {
	return p.getLogs(parameters)
}

func getReportFrequency(api plugin.API) (time.Duration, error) {
	b, err := api.KVGet(reportFrequencyKey)
	if err != nil {
		return 0, fmt.Errorf("api.KVGet: %w", err)
	}

	var minutes int

	if err := json.Unmarshal(b, &minutes); err != nil {
		return 0, fmt.Errorf("json.Unmarshal: %w", err)
	}

	return time.Duration(minutes) * time.Minute, nil
}

func (p *Plugin) setReportFrequency(parameters []string) string {
	if len(parameters) != 1 {
		return "You must supply exactly one parameter (number of minutes)."
	}

	i, err := strconv.Atoi(parameters[0])
	if err != nil {
		p.API.LogError("Atoi", logErrorKey, err)
		return settingReportFrequencyUnsuccessful
	}

	if i <= 0 {
		return "Invalid argument - report frequency must be a positive integer."
	}

	b, err := json.Marshal(i)
	if err != nil {
		p.API.LogError("Marshal", logErrorKey, err)
		return settingReportFrequencyUnsuccessful
	}

	if err := p.API.KVSet(reportFrequencyKey, b); err != nil {
		p.API.LogError("KVSet", logErrorKey, err)
		return settingReportFrequencyUnsuccessful
	}

	return "Report frequency set successfully."
}

func setReportFrequency(p *Plugin, channelID string, parameters []string) string {
	return p.setReportFrequency(parameters)
}

// func getReportChannel(api plugin.API) (string, error) {
// 	b, err := api.KVGet(reportChannelKey)
// 	if err != nil {
// 		return "", fmt.Errorf("api.KVGet: %w", err)
// 	}
//
// 	if b == nil {
// 		return "", nil
// 	}
//
// 	var channel string
//
// 	if err := json.Unmarshal(b, &channel); err != nil {
// 		return "", fmt.Errorf("json.Unmarshal: %w", err)
// 	}
//
// 	return channel, nil
// }

func setReportChannel(api plugin.API, channelID string) string {
	b, err := json.Marshal(channelID)
	if err != nil {
		api.LogError("Marshal", logErrorKey, err)
		return settingReportChannelUnsuccessful
	}

	if err := api.KVSet(reportChannelKey, b); err != nil {
		api.LogError("KVSet", logErrorKey, err)
		return settingReportChannelUnsuccessful
	}

	return "Subscribed to system monitoring report successfully."
}

func getChangesChannel(api plugin.API) (string, error) {
	b, err := api.KVGet(changesChannelKey)
	if err != nil {
		return "", fmt.Errorf("api.KVGet: %w", err)
	}

	if b == nil {
		return "", nil
	}

	var channel string

	if err := json.Unmarshal(b, &channel); err != nil {
		return "", fmt.Errorf("json.Unmarshal: %w", err)
	}

	return channel, nil
}

func setChangesChannel(api plugin.API, channelID string) string {
	b, err := json.Marshal(channelID)
	if err != nil {
		api.LogError("Marshal", logErrorKey, err)
		return settingChangesChannelUnsuccessful
	}

	if err := api.KVSet(changesChannelKey, b); err != nil {
		api.LogError("KVSet", logErrorKey, err)
		return settingChangesChannelUnsuccessful
	}

	return "Subscribed to configuration changes successfully."
}

func (p *Plugin) subscribe(channelID string, parameters []string) string {
	if len(parameters) != 1 {
		return "You must supply exactly one parameter (report|configuration-changes)."
	}

	switch parameters[0] {
	case "report":
		// TODO(amwolff): rewrite it to support HA (should be quick).
		stop := make(chan bool, 1)

		go p.addMonitoringReport(channelID, stop)

		p.subscriptionStop = stop

		return setReportChannel(p.API, channelID)
	case "configuration-changes":
		return setChangesChannel(p.API, channelID)
	default:
		return unknownParameterMessage(parameters[0])
	}
}

func subscribe(p *Plugin, channelID string, parameters []string) string {
	return p.subscribe(channelID, parameters)
}

func (p *Plugin) unsubscribe(parameters []string) string {
	if len(parameters) != 1 {
		return "You must supply exactly one parameter (report|configuration-changes)."
	}

	const unsubscribingUnsuccessful = "Unsubscribing unsuccessful."

	switch parameters[0] {
	case "report":
		// TODO(amwolff): rewrite it to support HA (should be quick).
		p.subscriptionStop <- true

		if err := p.API.KVDelete(reportChannelKey); err != nil {
			p.API.LogError("KVDelete", logErrorKey, err)
			return unsubscribingUnsuccessful
		}
	case "configuration-changes":
		if err := p.API.KVDelete(changesChannelKey); err != nil {
			p.API.LogError("KVDelete", logErrorKey, err)
			return unsubscribingUnsuccessful
		}
	default:
		return unknownParameterMessage(parameters[0])
	}

	return "Unsubscribed successfully."
}

func unsubscribe(p *Plugin, channelID string, parameters []string) string {
	return p.unsubscribe(parameters)
}
